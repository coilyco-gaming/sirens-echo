package community

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// The reference half of a skill, read when it is relevant rather than every
// turn. See docs/sirens-echo-prompt.md.

const (
	skillToolServer = "skills"
	skillToolName   = "read_skill"
)

// SkillProvider serves the references the prompt left out. In process, because
// the files are already on disk beside the binary.
type SkillProvider struct {
	References []SkillReference
}

func (p *SkillProvider) Open(context.Context) (ToolSession, error) {
	if p == nil || len(p.References) == 0 {
		return &skillSession{}, nil
	}
	byPath := make(map[string]SkillReference, len(p.References))
	for _, reference := range p.References {
		byPath[reference.Path] = reference
	}
	return &skillSession{byPath: byPath}, nil
}

type skillSession struct {
	byPath map[string]SkillReference
}

func (s *skillSession) Close() error { return nil }

// Grounding is empty. A reference reaches the turn because the model asked,
// which is the whole point of moving it out of the prompt.
func (s *skillSession) Grounding() []GroundingDocument { return nil }

func (s *skillSession) Guidance() []ServerGuidance { return nil }

func (s *skillSession) Unavailable() []string { return nil }

func (s *skillSession) Tools() []ToolDefinition {
	if len(s.byPath) == 0 {
		return nil
	}
	return []ToolDefinition{{
		Name:     skillToolName,
		Original: skillToolName,
		Server:   skillToolServer,
		Description: "Read one of the reference files listed under Readable references " +
			"in the local policy. Call it before answering on a subject one of them " +
			"names. Takes the path exactly as listed and reads nothing else.",
		InputSchema: scratchObjectSchema(map[string]any{
			"path": scratchStringProperty("Reference path exactly as the policy lists it."),
		}, []string{"path"}),
	}}
}

// Call returns one reference. An unknown path refuses with what is available,
// because a model that guessed once will otherwise guess again.
func (s *skillSession) Call(
	_ context.Context,
	name string,
	arguments map[string]any,
) (ToolResult, error) {
	if len(s.byPath) == 0 || name != skillToolName {
		return ToolResult{}, fmt.Errorf("model requested unavailable skill tool %q", name)
	}
	path := strings.TrimSpace(scratchStringArg(arguments, "path"))
	reference, known := s.byPath[path]
	if !known {
		return scratchRefusal(
			"no reference at %q. The readable ones are: %s",
			path, strings.Join(s.paths(), ", "))
	}
	return ToolResult{Text: reference.Body, Detail: skillDisplayName(reference.Path)}, nil
}

// skillDisplayName is the reference as a member sees it: the file's own name,
// taken from the validated path rather than from the argument.
func skillDisplayName(path string) string {
	base := path[strings.LastIndex(path, "/")+1:]
	if dot := strings.LastIndex(base, "."); dot > 0 {
		base = base[:dot]
	}
	return base
}

func (s *skillSession) paths() []string {
	paths := make([]string, 0, len(s.byPath))
	for path := range s.byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
