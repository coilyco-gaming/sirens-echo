package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

// MCPProvider connects a definition's source-controlled MCP roster through the
// official Go SDK. An empty roster is a valid no-tool capability boundary.
type MCPProvider struct {
	Servers    []MCPServerDefinition
	HTTPClient *http.Client
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

// Open initializes every configured MCP server and discovers its complete tool
// list before Agent Proxy chooses a tool.
func (p MCPProvider) Open(ctx context.Context) (ToolSession, error) {
	client := mcp.NewClient(
		&mcp.Implementation{Name: "sirens-echo", Version: "1"},
		nil,
	)
	opened := &mcpToolSession{
		registered: make(map[string]registeredMCPTool),
	}
	for _, server := range p.Servers {
		// A server that cannot answer contributes no tools and the turn goes on
		// with the rest. One transient outage must not cost every turn.
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           p.HTTPClient,
			DisableStandaloneSSE: true,
		}, nil)
		if err != nil {
			opened.unavailable = append(opened.unavailable, server.Name)
			continue
		}
		discovered, err := discoverTools(ctx, session)
		if err != nil {
			_ = session.Close()
			opened.unavailable = append(opened.unavailable, server.Name)
			continue
		}
		opened.sessions = append(opened.sessions, session)
		// A collision stays fatal. That is a roster mistake, and degrading past
		// it would silently drop whichever tool lost the race.
		if err := opened.register(server.Name, session, discovered); err != nil {
			_ = opened.Close()
			return nil, err
		}
	}
	if len(p.Servers) > 0 && len(opened.unavailable) == len(p.Servers) {
		_ = opened.Close()
		return nil, fmt.Errorf("no configured MCP server is reachable")
	}
	return opened, nil
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
