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
	tools      []ToolDefinition
	registered map[string]registeredMCPTool
	sessions   []*mcp.ClientSession
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
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint:             server.URL,
			HTTPClient:           p.HTTPClient,
			DisableStandaloneSSE: true,
		}, nil)
		if err != nil {
			_ = opened.Close()
			return nil, fmt.Errorf("connect MCP server %s: %w", server.Name, err)
		}
		opened.sessions = append(opened.sessions, session)
		cursor := ""
		for {
			result, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
			if err != nil {
				_ = opened.Close()
				return nil, fmt.Errorf("list MCP tools from %s: %w", server.Name, err)
			}
			for _, tool := range result.Tools {
				name, err := proxyToolName(server.Name, tool.Name)
				if err != nil {
					_ = opened.Close()
					return nil, err
				}
				if _, exists := opened.registered[name]; exists {
					_ = opened.Close()
					return nil, fmt.Errorf("MCP tool name collision %q", name)
				}
				opened.registered[name] = registeredMCPTool{
					serverName: server.Name,
					toolName:   tool.Name,
					session:    session,
				}
				opened.tools = append(opened.tools, ToolDefinition{
					Name:        name,
					Server:      server.Name,
					Original:    tool.Name,
					Description: tool.Description,
					InputSchema: tool.InputSchema,
				})
			}
			if result.NextCursor == "" {
				break
			}
			cursor = result.NextCursor
		}
	}
	return opened, nil
}

func (s *mcpToolSession) Tools() []ToolDefinition {
	return append([]ToolDefinition(nil), s.tools...)
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
