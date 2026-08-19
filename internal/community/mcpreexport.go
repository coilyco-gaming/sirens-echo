package community

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Re-exporting the roster puts a caller past runReplyChecks, response
// validation, and IdentifierGuard, all of which live in the turn pipeline
// rather than in the tools. That is a security boundary moving rather than an
// interface being added, so this is off unless a deployment opts in and every
// call is refused without the deployment token. sirens-echo#1025 records the
// decision and what it reverses.

// reexportUntrusted is returned instead of a result, and says which of the two
// reasons applies without revealing whether a token was configured at all.
const reexportUntrusted = "this tool requires the deployment token. " +
	"Present it as an Authorization Bearer header, or call turn instead, " +
	"which runs the same work through the deployment's response checks."

// reexportDescription prefixes the upstream server's own text so a client can
// see that it reached a tool directly rather than through a turn.
func reexportDescription(definition ToolDefinition) string {
	upstream := strings.TrimSpace(definition.Description)
	prefix := fmt.Sprintf("[%s, called directly]", definition.Server)
	if upstream == "" {
		return prefix + " No description offered by the server."
	}
	return prefix + " " + upstream
}

// reexportCache holds the last discovered roster so tools/list does not open a
// session per request. A call always opens its own.
type reexportCache struct {
	mu      sync.Mutex
	tools   []ToolDefinition
	fetched time.Time
	// provider substitutes the roster in tests. Nil uses the real one, so a
	// deployment cannot reach this field.
	provider ToolProvider
}

// rosterProvider is the seam the re-export opens through. a.tools is a concrete
// *MCPProvider, and returning it directly as an interface would hand back a
// non-nil interface holding a nil pointer.
func (a *Agent) rosterProvider() ToolProvider {
	if a.reexport.provider != nil {
		return a.reexport.provider
	}
	if a.tools == nil {
		return nil
	}
	return a.tools
}

// snapshot returns the advertised roster, refreshing when stale. A discovery
// failure keeps the previous list rather than emptying it, because a client
// whose tool list vanishes underneath it is the failure #943 recorded.
func (a *Agent) reexportSnapshot(ctx context.Context) []ToolDefinition {
	a.reexport.mu.Lock()
	defer a.reexport.mu.Unlock()
	provider := a.rosterProvider()
	if provider == nil {
		return nil
	}
	if a.reexport.tools != nil && time.Since(a.reexport.fetched) < reexportRefreshInterval {
		return a.reexport.tools
	}
	session, err := provider.Open(ctx)
	if err != nil {
		return a.reexport.tools
	}
	defer func() { _ = session.Close() }()
	discovered := session.Tools()
	sort.Slice(discovered, func(i, j int) bool {
		return discovered[i].Name < discovered[j].Name
	})
	a.reexport.tools = discovered
	a.reexport.fetched = time.Now()
	return a.reexport.tools
}

// headerTrusted is callerTrusted against a header map, because an MCP tool call
// reaches its headers through the SDK request rather than an *http.Request.
func headerTrusted(header http.Header, configured string) bool {
	if configured == "" || header == nil {
		return false
	}
	value := header.Get(httpTrustHeader)
	if !strings.HasPrefix(value, httpTrustScheme) {
		return false
	}
	presented := strings.TrimSpace(strings.TrimPrefix(value, httpTrustScheme))
	return subtle.ConstantTimeCompare([]byte(presented), []byte(configured)) == 1
}

func reexportRequestTrusted(request *mcp.CallToolRequest, configured string) bool {
	if request == nil || request.Extra == nil {
		return false
	}
	return headerTrusted(request.Extra.Header, configured)
}

// registerReexportedTools adds one MCP tool per rostered tool. Names arrive
// already namespaced as server__tool from proxyToolName, so the collision rule
// that guards the model's tool list guards this surface too.
func (a *Agent) registerReexportedTools(server *mcp.Server, ctx context.Context) {
	for _, definition := range a.reexportSnapshot(ctx) {
		if definition.Name == "turn" {
			// The harness owns that name on this surface.
			continue
		}
		a.addReexportedTool(server, definition)
	}
}

// addReexportedTool binds one definition. Taking it by value is what keeps each
// handler bound to its own tool rather than to the loop's last one.
func (a *Agent) addReexportedTool(server *mcp.Server, definition ToolDefinition) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        definition.Name,
			Description: reexportDescription(definition),
			InputSchema: reexportSchema(definition.InputSchema),
		},
		func(
			ctx context.Context,
			request *mcp.CallToolRequest,
			arguments map[string]any,
		) (*mcp.CallToolResult, map[string]any, error) {
			return a.handleReexportedCall(ctx, request, definition, arguments)
		},
	)
}

func (a *Agent) handleReexportedCall(
	ctx context.Context,
	request *mcp.CallToolRequest,
	definition ToolDefinition,
	arguments map[string]any,
) (*mcp.CallToolResult, map[string]any, error) {
	if !reexportRequestTrusted(request, a.cfg.HTTPTrustToken) {
		return toolFailure(reexportUntrusted), nil, nil
	}

	// Tool calls are cheaper than turns and some of them write, so they are
	// admitted under the same budget rather than beside it. sirens-echo#1025.
	decision := a.limiter.Admit(admissionRequest{
		UserKey:    mcpPrincipal(request),
		ContextKey: transportMCP,
		Queued:     true,
	})
	if decision.Outcome.denied() {
		a.telemetry.RecordAdmission(ctx, string(decision.Outcome), transportMCP)
		return toolFailure("rate limit reached"), nil, nil
	}
	defer a.limiter.Release()
	a.telemetry.RecordAdmission(ctx, string(admissionAccepted), transportMCP)

	provider := a.rosterProvider()
	if provider == nil {
		return toolFailure("no MCP server is configured for this deployment"), nil, nil
	}
	session, err := provider.Open(ctx)
	if err != nil {
		return toolFailure("the tool roster could not be read"), nil, nil
	}
	defer func() { _ = session.Close() }()

	result, err := session.Call(ctx, definition.Name, arguments)
	if err != nil {
		return toolFailure(err.Error()), nil, nil
	}
	if result.IsError {
		return toolFailure(result.Text), nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result.Text}},
	}, nil, nil
}

// reexportSchema passes the upstream schema through when it is an object the
// SDK can serve, and otherwise offers a free-form object. An upstream that
// describes its arguments oddly should not remove the tool.
func reexportSchema(schema any) any {
	if schema == nil {
		return map[string]any{"type": "object"}
	}
	return schema
}
