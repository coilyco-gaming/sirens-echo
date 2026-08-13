package community

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// One tool surface assembled from several providers. See docs/sirens-echo-tools.md.

// CompositeProvider opens several tool providers as one session, so in-process
// capabilities and the MCP roster reach the model in a single tools array.
type CompositeProvider struct {
	Providers []ToolProvider
}

// Open opens every provider, closing the ones already open on failure so a
// half-open session cannot leak MCP connections past the turn.
func (p *CompositeProvider) Open(ctx context.Context) (ToolSession, error) {
	opened := make([]ToolSession, 0, len(p.Providers))
	for _, provider := range p.Providers {
		if provider == nil {
			continue
		}
		session, err := provider.Open(ctx)
		if err != nil {
			closeSessions(opened)
			return nil, err
		}
		opened = append(opened, session)
	}
	combined := &compositeSession{sessions: opened, owner: make(map[string]ToolSession)}
	if err := combined.index(); err != nil {
		closeSessions(opened)
		return nil, err
	}
	return combined, nil
}

func closeSessions(sessions []ToolSession) {
	for _, session := range sessions {
		_ = session.Close()
	}
}

type compositeSession struct {
	sessions []ToolSession
	tools    []ToolDefinition
	owner    map[string]ToolSession
}

// index fails on a name collision rather than choosing a winner. Two tools
// answering to one name is a routing decision nobody made.
func (s *compositeSession) index() error {
	for _, session := range s.sessions {
		for _, definition := range session.Tools() {
			if _, exists := s.owner[definition.Name]; exists {
				return fmt.Errorf("tool name collision %q", definition.Name)
			}
			// Telemetry reports the original name, so a provider that leaves it
			// unset goes unnamed in the tool breakdown rather than failing.
			if strings.TrimSpace(definition.Original) == "" {
				definition.Original = definition.Name
			}
			s.owner[definition.Name] = session
			s.tools = append(s.tools, definition)
		}
	}
	return nil
}

func (s *compositeSession) Tools() []ToolDefinition {
	return append([]ToolDefinition(nil), s.tools...)
}

func (s *compositeSession) Grounding() []GroundingDocument {
	var grounding []GroundingDocument
	for _, session := range s.sessions {
		grounding = append(grounding, session.Grounding()...)
	}
	return grounding
}

func (s *compositeSession) Unavailable() []string {
	var unavailable []string
	for _, session := range s.sessions {
		unavailable = append(unavailable, session.Unavailable()...)
	}
	return unavailable
}

func (s *compositeSession) Call(
	ctx context.Context,
	name string,
	arguments map[string]any,
) (ToolResult, error) {
	session, exists := s.owner[name]
	if !exists {
		return ToolResult{}, fmt.Errorf("model requested unavailable tool %q", name)
	}
	return session.Call(ctx, name, arguments)
}

// Close closes every session and reports the failures together, so one
// stubborn transport cannot hide another.
func (s *compositeSession) Close() error {
	var failures []error
	for _, session := range s.sessions {
		if err := session.Close(); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
