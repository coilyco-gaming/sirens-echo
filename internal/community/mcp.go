package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"
)

const maxProxyToolNameBytes = 64

var invalidProxyToolName = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// ToolDefinition is one MCP tool translated to Agent Proxy's function schema.
type ToolDefinition struct {
	Name        string
	Server      string
	Original    string
	Description string
	InputSchema any
}

// ToolResult is one completed MCP tool call rendered for the model. Bounding
// text cannot corrupt a structure the way cutting a marshalled envelope did.
type ToolResult struct {
	Text    string
	IsError bool
}

// ToolSession is the per-turn MCP capability available to Agent Proxy.
type ToolSession interface {
	Tools() []ToolDefinition
	Unavailable() []string
	Call(ctx context.Context, name string, arguments map[string]any) (ToolResult, error)
	Close() error
}

// ToolProvider opens the configured MCP roster for one model turn.
type ToolProvider interface {
	Open(ctx context.Context) (ToolSession, error)
}

const (
	// defaultRosterRefresh bounds staleness for a transport that cannot push
	// tools/list_changed. See docs/sirens-echo-mcp-roster.md.
	defaultRosterRefresh = 5 * time.Minute
	mcpConnectTimeout    = 10 * time.Second
	mcpListTimeout       = 15 * time.Second
	mcpBackoffMin        = 5 * time.Second
	mcpBackoffMax        = 2 * time.Minute
)

// MCPProvider supervises the configured MCP roster through the official Go SDK.
// An empty roster is a valid no-tool capability boundary.
type MCPProvider struct {
	Servers    []MCPServerDefinition
	HTTPClient *http.Client
	// RefreshInterval bounds staleness where notifications cannot arrive. Zero
	// uses defaultRosterRefresh.
	RefreshInterval time.Duration

	mu      sync.Mutex
	started bool
	root    context.Context
	cancel  context.CancelFunc
	entries []*supervisedServer
}

// supervisedServer holds one server's connection across turns, so a turn pays
// no handshake and a tool listing survives until something invalidates it.
type supervisedServer struct {
	definition MCPServerDefinition
	notifies   bool
	session    *mcp.ClientSession
	tools      []*mcp.Tool
	refreshed  time.Time
	retryAfter time.Time
	backoff    time.Duration
	stale      atomic.Bool
}

// transportNotifies reports whether a transport can deliver server-initiated
// messages. Streamable cannot while its standalone SSE stream stays disabled.
func transportNotifies(server MCPServerDefinition) bool {
	switch server.ResolvedTransport() {
	case MCPTransportStdio, MCPTransportSSE:
		return true
	}
	return false
}

type registeredMCPTool struct {
	serverName string
	toolName   string
	session    *mcp.ClientSession
}

type mcpToolSession struct {
	tools       []ToolDefinition
	registered  map[string]registeredMCPTool
	sessions    []*mcp.ClientSession
	unavailable []string
}

// Open returns this turn's view over the supervised roster. It connects only
// what is not already connected and lists only what is stale or expired.
func (p *MCPProvider) Open(ctx context.Context) (ToolSession, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startLocked()
	opened := &mcpToolSession{
		registered: make(map[string]registeredMCPTool),
	}
	now := time.Now()
	for _, entry := range p.entries {
		// A server that cannot answer contributes no tools and the turn goes on
		// with the rest. One transient outage must not cost every turn.
		if err := p.readyLocked(ctx, entry, now); err != nil {
			opened.unavailable = append(opened.unavailable, entry.definition.Name)
			continue
		}
		// A collision stays fatal. That is a roster mistake, and degrading past
		// it would silently drop whichever tool lost the race.
		if err := opened.register(entry.definition.Name, entry.session, entry.tools); err != nil {
			return nil, err
		}
	}
	if len(p.entries) > 0 && len(opened.unavailable) == len(p.entries) {
		return nil, fmt.Errorf("no configured MCP server is reachable")
	}
	return opened, nil
}

func (p *MCPProvider) startLocked() {
	if p.started {
		return
	}
	p.started = true
	// Detached from any turn, because these connections outlive the turn that
	// first needed them.
	p.root, p.cancel = context.WithCancel(context.Background())
	for _, server := range p.Servers {
		p.entries = append(p.entries, &supervisedServer{
			definition: server,
			notifies:   transportNotifies(server),
		})
	}
}

// readyLocked connects and lists only what this turn actually needs.
func (p *MCPProvider) readyLocked(
	turnCtx context.Context,
	entry *supervisedServer,
	now time.Time,
) error {
	base := p.turnTraced(turnCtx)
	if entry.session == nil {
		if now.Before(entry.retryAfter) {
			return fmt.Errorf("MCP server %s is backing off", entry.definition.Name)
		}
		if err := p.connectLocked(base, entry); err != nil {
			entry.penalise(now)
			return err
		}
		entry.backoff = 0
	}
	if !entry.needsTools(p.refreshInterval(), now) {
		return nil
	}
	listCtx, cancel := context.WithTimeout(base, mcpListTimeout)
	defer cancel()
	discovered, err := discoverTools(listCtx, entry.session)
	if err != nil {
		_ = entry.session.Close()
		entry.session = nil
		entry.tools = nil
		entry.penalise(now)
		return err
	}
	entry.tools = discovered
	entry.refreshed = now
	return nil
}

// turnTraced cancels with the pool but carries the calling turn's span context,
// so a pooled connection outlives the turn while its traffic stays correlated.
func (p *MCPProvider) turnTraced(turnCtx context.Context) context.Context {
	return trace.ContextWithSpanContext(p.root, trace.SpanContextFromContext(turnCtx))
}

func (p *MCPProvider) connectLocked(base context.Context, entry *supervisedServer) error {
	// One client per server, so its notification handler needs no way to map a
	// request back to which server sent it.
	client := mcp.NewClient(
		&mcp.Implementation{Name: "sirens-echo", Version: "1"},
		&mcp.ClientOptions{
			ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
				entry.stale.Store(true)
			},
		},
	)
	transport, err := clientTransport(base, entry.definition, p.HTTPClient)
	if err != nil {
		return err
	}
	connectCtx, cancel := context.WithTimeout(base, mcpConnectTimeout)
	defer cancel()
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return err
	}
	entry.session = session
	entry.tools = nil
	entry.stale.Store(false)
	return nil
}

func (p *MCPProvider) refreshInterval() time.Duration {
	if p.RefreshInterval > 0 {
		return p.RefreshInterval
	}
	return defaultRosterRefresh
}

// needsTools is true on a first listing, on a list_changed notification, and on
// expiry for a transport that cannot send one.
func (s *supervisedServer) needsTools(interval time.Duration, now time.Time) bool {
	if s.tools == nil || s.stale.Swap(false) {
		return true
	}
	return !s.notifies && now.Sub(s.refreshed) >= interval
}

func (s *supervisedServer) penalise(now time.Time) {
	if s.backoff == 0 {
		s.backoff = mcpBackoffMin
	} else if s.backoff < mcpBackoffMax {
		s.backoff *= 2
	}
	if s.backoff > mcpBackoffMax {
		s.backoff = mcpBackoffMax
	}
	s.retryAfter = now.Add(s.backoff)
}

// Close shuts down every supervised connection. A stdio child exits with the
// root context rather than outliving the process that started it.
func (p *MCPProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	errs := make([]error, 0, len(p.entries))
	for _, entry := range p.entries {
		if entry.session == nil {
			continue
		}
		if err := entry.session.Close(); err != nil {
			errs = append(errs, err)
		}
		entry.session = nil
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.entries = nil
	p.started = false
	return errors.Join(errs...)
}

// clientTransport builds the transport one roster entry asks for. Validation
// has already rejected an entry whose fields do not match its transport.
func clientTransport(
	ctx context.Context,
	server MCPServerDefinition,
	httpClient *http.Client,
) (mcp.Transport, error) {
	switch server.ResolvedTransport() {
	case MCPTransportStreamable:
		return &mcp.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           httpClient,
			DisableStandaloneSSE: true,
		}, nil
	case MCPTransportSSE:
		return &mcp.SSEClientTransport{
			Endpoint:   server.URL,
			HTTPClient: httpClient,
		}, nil
	case MCPTransportStdio:
		// Bound to the caller's context, so the child dies with the session
		// rather than outliving it.
		command := exec.CommandContext(ctx, server.Command, server.Args...)
		command.Env = environSlice(server.Env)
		return &mcp.CommandTransport{Command: command}, nil
	}
	return nil, fmt.Errorf(
		"MCP server %s has unsupported transport %q",
		server.Name,
		server.ResolvedTransport(),
	)
}

// environSlice renders the roster's declared environment for a stdio child.
// Echo's own variables are never inherited, so nothing rides along by accident.
func environSlice(env map[string]string) []string {
	pairs := make([]string, 0, len(env))
	for key, value := range env {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}

func discoverTools(ctx context.Context, session *mcp.ClientSession) ([]*mcp.Tool, error) {
	discovered := make([]*mcp.Tool, 0)
	cursor := ""
	for {
		result, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		discovered = append(discovered, result.Tools...)
		if result.NextCursor == "" {
			return discovered, nil
		}
		cursor = result.NextCursor
	}
}

func (s *mcpToolSession) register(
	serverName string,
	session *mcp.ClientSession,
	discovered []*mcp.Tool,
) error {
	for _, tool := range discovered {
		name, err := proxyToolName(serverName, tool.Name)
		if err != nil {
			return err
		}
		if _, exists := s.registered[name]; exists {
			return fmt.Errorf("MCP tool name collision %q", name)
		}
		s.registered[name] = registeredMCPTool{
			serverName: serverName,
			toolName:   tool.Name,
			session:    session,
		}
		s.tools = append(s.tools, ToolDefinition{
			Name:        name,
			Server:      serverName,
			Original:    tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return nil
}

func (s *mcpToolSession) Tools() []ToolDefinition {
	return append([]ToolDefinition(nil), s.tools...)
}

// Unavailable names the configured servers that did not answer this turn.
func (s *mcpToolSession) Unavailable() []string {
	return append([]string(nil), s.unavailable...)
}

func (s *mcpToolSession) Call(
	ctx context.Context,
	name string,
	arguments map[string]any,
) (ToolResult, error) {
	tool, exists := s.registered[name]
	if !exists {
		return ToolResult{}, fmt.Errorf("model requested unavailable MCP tool %q", name)
	}
	result, err := tool.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tool.toolName,
		Arguments: arguments,
	})
	if err != nil {
		return ToolResult{}, fmt.Errorf(
			"call MCP tool %s/%s: %w", tool.serverName, tool.toolName, err,
		)
	}
	text, err := toolResultText(result)
	if err != nil {
		return ToolResult{}, fmt.Errorf(
			"render MCP tool result %s/%s: %w", tool.serverName, tool.toolName, err,
		)
	}
	return ToolResult{Text: text, IsError: result.IsError}, nil
}

// toolResultText renders a result as text. A non-text part keeps its JSON form
// rather than being dropped, which would read to the model as an empty answer.
func toolResultText(result *mcp.CallToolResult) (string, error) {
	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
			continue
		}
		raw, err := content.MarshalJSON()
		if err != nil {
			return "", err
		}
		parts = append(parts, string(raw))
	}
	if len(parts) == 0 && result.StructuredContent != nil {
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return "", err
		}
		parts = append(parts, string(raw))
	}
	return strings.Join(parts, "\n"), nil
}

func (s *mcpToolSession) Close() error {
	errs := make([]error, 0, len(s.sessions))
	for _, session := range s.sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func proxyToolName(server, tool string) (string, error) {
	name := invalidProxyToolName.ReplaceAllString(server+"__"+tool, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return "", fmt.Errorf("MCP tool %q/%q has no usable proxy name", server, tool)
	}
	if len(name) > maxProxyToolNameBytes {
		return "", fmt.Errorf(
			"MCP tool proxy name %q exceeds %d bytes",
			name,
			maxProxyToolNameBytes,
		)
	}
	return name, nil
}
