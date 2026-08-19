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

// A re-exported call reaches a server without the turn pipeline's checks, so
// it is gated rather than offered. See docs/sirens-echo-http.md, #1025.

const reexportUntrusted = "this tool requires the deployment token. " +
	"Present it as an Authorization Bearer header, or call turn instead, " +
	"which runs the same work through the deployment's response checks."

func reexportDescription(definition ToolDefinition) string {
	upstream := strings.TrimSpace(definition.Description)
	prefix := fmt.Sprintf("[%s, called directly]", definition.Server)
	if upstream == "" {
		return prefix + " No description offered by the server."
	}
	return prefix + " " + upstream
}

type reexportCache struct {
	mu      sync.Mutex
	tools   []ToolDefinition
	fetched time.Time
	// provider substitutes the roster in tests.
	provider ToolProvider
}

// rosterProvider guards the nil-interface trap: a.tools is a concrete type, so
// returning it directly yields a non-nil interface holding a nil pointer.
func (a *Agent) rosterProvider() ToolProvider {
	if a.reexport.provider != nil {
		return a.reexport.provider
	}
	if a.tools == nil {
		return nil
	}
	return a.tools
}

// reexportSnapshot keeps the previous list when discovery fails, because #943's
// collapse to zero is worse seen by a client than a stale list.
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

// registerReexportedTools leans on proxyToolName having already namespaced each
// name as server__tool, so the existing collision rule covers this surface.
func (a *Agent) registerReexportedTools(server *mcp.Server, ctx context.Context) {
	for _, definition := range a.reexportSnapshot(ctx) {
		if definition.Name == "turn" {
			// The harness owns that name here.
			continue
		}
		a.addReexportedTool(server, definition)
	}
}

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

// reexportSchema keeps a tool whose upstream schema is missing, rather than
// dropping it for describing its arguments oddly.
func reexportSchema(schema any) any {
	if schema == nil {
		return map[string]any{"type": "object"}
	}
	return schema
}
