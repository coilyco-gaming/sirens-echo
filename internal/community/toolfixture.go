package community

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FixtureSchema names the declared-results file an evaluation run may serve
// instead of the live roster. See docs/sirens-echo-tool-fixture.md.
const FixtureSchema = "sirens-discord-ops.tool-fixture.v1"

// FixtureTool is one tool the fixture offers and the single result it returns.
// Arguments are ignored, because the payload is the variable under test.
type FixtureTool struct {
	Name        string `yaml:"name"`
	Server      string `yaml:"server"`
	Description string `yaml:"description"`
	// Result is returned verbatim as the tool's text, so a case can place a
	// payload inside tool output rather than inside the caller's message.
	Result string `yaml:"result"`
	// IsError marks the result as a tool failure rather than a value.
	IsError bool `yaml:"is_error"`
}

// FixturePack is the tracked set of declared tools.
type FixturePack struct {
	Schema string        `yaml:"schema"`
	Tools  []FixtureTool `yaml:"tools"`
}

// FixtureProvider serves declared results in process. It opens no socket and
// starts no process, so a case using it cannot reach a live deployment.
type FixtureProvider struct {
	Pack FixturePack
}

// LoadFixturePack reads and validates a declared-results file.
func LoadFixturePack(path string) (FixturePack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FixturePack{}, fmt.Errorf("read tool fixture: %w", err)
	}
	var pack FixturePack
	if err := yaml.Unmarshal(raw, &pack); err != nil {
		return FixturePack{}, fmt.Errorf("parse tool fixture: %w", err)
	}
	if pack.Schema != FixtureSchema {
		return FixturePack{}, fmt.Errorf("unsupported tool fixture schema %q", pack.Schema)
	}
	if len(pack.Tools) == 0 {
		return FixturePack{}, fmt.Errorf("tool fixture declares no tools")
	}
	seen := make(map[string]struct{}, len(pack.Tools))
	for _, tool := range pack.Tools {
		if tool.Name == "" || tool.Server == "" {
			return FixturePack{}, fmt.Errorf("fixture tool requires name and server")
		}
		if _, duplicate := seen[tool.Name]; duplicate {
			return FixturePack{}, fmt.Errorf("fixture declares %s twice", tool.Name)
		}
		seen[tool.Name] = struct{}{}
	}
	return pack, nil
}

// Open returns a session over the declared tools. The context is unused
// because nothing is dialled.
func (p FixtureProvider) Open(_ context.Context) (ToolSession, error) {
	return fixtureSession{pack: p.Pack}, nil
}

type fixtureSession struct {
	pack FixturePack
}

func (s fixtureSession) Tools() []ToolDefinition {
	tools := make([]ToolDefinition, 0, len(s.pack.Tools))
	for _, tool := range s.pack.Tools {
		tools = append(tools, ToolDefinition{
			Name:        tool.Name,
			Server:      tool.Server,
			Original:    tool.Name,
			Description: tool.Description,
			InputSchema: map[string]any{"type": "object"},
		})
	}
	return tools
}

// Grounding is empty. A fixture declares tool results, and a grounding
// document is a different surface with its own case shape.
func (s fixtureSession) Grounding() []GroundingDocument { return nil }

// Unavailable is empty. A declared tool is always available, so an absent
// result is a fixture authoring error rather than a degraded roster.
func (s fixtureSession) Unavailable() []string { return nil }

// Call returns the declared result. Arguments are ignored on purpose: varying
// a result by argument would make the payload conditional and the case fragile.
func (s fixtureSession) Call(
	_ context.Context,
	name string,
	_ map[string]any,
) (ToolResult, error) {
	for _, tool := range s.pack.Tools {
		if tool.Name == name {
			return ToolResult{Text: tool.Result, IsError: tool.IsError}, nil
		}
	}
	return ToolResult{}, fmt.Errorf("tool fixture declares no result for %s", name)
}

func (s fixtureSession) Close() error { return nil }
