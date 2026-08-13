package community

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
)

// A read-only fetch, bounded by an allowlist the deployment supplies. See
// docs/sirens-echo-fetch.md.

const (
	fetchToolServer = "fetch"
	fetchToolName   = "fetch_url"
)

// FetchProvider exposes an HTTPS GET as a tool. An empty Hosts offers no tool
// at all, rather than a tool that refuses everything.
type FetchProvider struct {
	Hosts  []string
	Client *http.Client
}

// Open needs no per-turn state. The bound is the allowlist, not the requester.
func (p *FetchProvider) Open(context.Context) (ToolSession, error) {
	if p == nil || len(p.Hosts) == 0 {
		return &fetchSession{}, nil
	}
	client := p.Client
	if client == nil {
		client = newFetchClient()
	}
	return &fetchSession{hosts: p.Hosts, client: client}, nil
}

type fetchSession struct {
	hosts  []string
	client *http.Client
}

func (s *fetchSession) Close() error { return nil }

// Grounding is empty: a fetch answers a question rather than volunteering
// reference material the way a server's resources do.
func (s *fetchSession) Grounding() []GroundingDocument { return nil }

func (s *fetchSession) Unavailable() []string { return nil }

func (s *fetchSession) Tools() []ToolDefinition {
	if len(s.hosts) == 0 {
		return nil
	}
	return []ToolDefinition{{
		Name:     fetchToolName,
		Original: fetchToolName,
		Server:   fetchToolServer,
		Description: "Fetch a page over HTTPS and return its text. Only these hosts " +
			"are reachable: " + strings.Join(s.hosts, ", ") + ". Nothing else is.",
		InputSchema: scratchObjectSchema(map[string]any{
			"url": scratchStringProperty("Full https URL to fetch."),
		}, []string{"url"}),
	}}
}

func (s *fetchSession) Call(
	ctx context.Context,
	name string,
	arguments map[string]any,
) (ToolResult, error) {
	if len(s.hosts) == 0 || name != fetchToolName {
		return ToolResult{}, fmt.Errorf("model requested unavailable fetch tool %q", name)
	}
	target, err := s.allowedURL(scratchStringArg(arguments, "url"))
	if err != nil {
		return ToolResult{Text: err.Error(), IsError: true}, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return ToolResult{Text: "that url cannot be requested", IsError: true}, nil
	}
	response, err := s.client.Do(request)
	if err != nil {
		return ToolResult{Text: "that host did not answer", IsError: true}, nil
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, int64(maxFetchBytes)))
	if err != nil {
		return ToolResult{Text: "that response could not be read", IsError: true}, nil
	}
	return ToolResult{Text: fmt.Sprintf("%d\n%s", response.StatusCode, string(body))}, nil
}

// allowedURL refuses anything the allowlist does not name. The host is matched
// exactly, so a lookalike domain cannot pass as a suffix.
func (s *fetchSession) allowedURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("that url cannot be parsed")
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("only https is available, not %q", parsed.Scheme)
	}
	host := strings.ToLower(parsed.Hostname())
	for _, allowed := range s.hosts {
		if host == strings.ToLower(allowed) {
			return parsed.String(), nil
		}
	}
	return "", fmt.Errorf("%s is not a host this service may reach", host)
}

// newFetchClient refuses a private destination at dial time. Checking the
// hostname is not enough. See docs/sirens-echo-fetch.md.
func newFetchClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: fetchTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			return refusePrivateAddress(address)
		},
	}
	return &http.Client{
		Timeout:   fetchTimeout,
		Transport: &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			// A redirect is a second destination the allowlist never saw.
			return fmt.Errorf("refusing a redirect to %s", request.URL.Hostname())
		},
	}
}

// tailnetRange is carrier-grade NAT, which Tailscale assigns from and which
// IsPrivate does not cover. See docs/sirens-echo-fetch.md.
var tailnetRange = func() *net.IPNet {
	_, network, _ := net.ParseCIDR("100.64.0.0/10")
	return network
}()

// refusePrivateAddress rejects loopback, link-local, and private ranges, which
// is where a cluster keeps everything worth reaching.
func refusePrivateAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("unreadable address %q", address)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("unresolved address %q", host)
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() ||
		tailnetRange.Contains(ip) {
		return fmt.Errorf("refusing an internal address")
	}
	return nil
}

// fetchHosts reads the deployment's allowlist. Empty offers no tool.
func fetchHosts(raw string) []string {
	hosts := make([]string, 0)
	for _, field := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			hosts = append(hosts, trimmed)
		}
	}
	return hosts
}
