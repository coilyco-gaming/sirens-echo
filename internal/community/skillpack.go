package community

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skillEntrypoints are the body filenames a policy root may use. Ordinary
// skills ship SKILL.md and agent-compose composed sources ship COMPOSED.md.
var skillEntrypoints = []string{"SKILL.md", "COMPOSED.md"}

// LoadSkillpack loads each root's entrypoint and one-level reference Markdown
// in deterministic order. See docs/sirens-echo-prompt.md.
func LoadSkillpack(roots []string) (string, error) {
	files := make([]string, 0)
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			slashed := filepath.ToSlash(relative)
			if isSkillEntrypoint(slashed) ||
				(strings.HasPrefix(slashed, "references/") && strings.HasSuffix(slashed, ".md")) {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return "", fmt.Errorf("walk skill root %s: %w", root, err)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return "", fmt.Errorf("skillpack contains no SKILL.md or reference Markdown")
	}

	var output strings.Builder
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read skill file %s: %w", path, err)
		}
		body := stripFrontmatter(string(raw))
		if body == "" {
			return "", fmt.Errorf("skill file %s has no body", path)
		}
		fmt.Fprintf(&output, "\n## Source: %s\n\n%s\n", filepath.ToSlash(path), body)
		if output.Len() > maxSkillpackBytes {
			return "", fmt.Errorf("skillpack exceeds %d bytes", maxSkillpackBytes)
		}
	}
	return strings.TrimSpace(output.String()), nil
}

// PlaceholderComposed keeps the tracked snapshot and the build-time policy
// check hermetic. It carries the surface the validator anchors on.
const PlaceholderComposed = `# Role instructions

Agent-compose assigned the ` + "`<role>`" + ` role from the caller's compose request.

**Role skill // ` + "`role-<role>`" + `**
**Agent // <seat>**

## Personality meld

Placeholder. Deployment selects the role and the image bakes the real bundle.`

// composedForRun returns the bundle a run reads and the label its dataset
// records together, so a dataset cannot name a bundle the run never read.
func composedForRun(definition Definition) (bundle string, recorded string, err error) {
	if !definition.Composed {
		return "", ComposedNotRequested, nil
	}
	dir := strings.TrimSpace(os.Getenv(ComposedBundleEnv))
	if dir == "" {
		return PlaceholderComposed, ComposedStubbed, nil
	}
	// A failure must not fall back to the stub. A dataset labelled with a bundle
	// it did not read is worse than one honestly labelled stubbed.
	loaded, loadErr := LoadBundle(dir)
	if loadErr != nil {
		return "", "", fmt.Errorf("%s=%s: %w", ComposedBundleEnv, dir, loadErr)
	}
	return loaded, fmt.Sprintf("bundle %s (%d bytes)", dir, len(loaded)), nil
}

// LoadBundle reads one materialized agent-compose bundle: the identity card
// plus every selected skill as its own policy root. See docs/sirens-echo-compose.md.
func LoadBundle(dir string) (string, error) {
	card, err := os.ReadFile(filepath.Join(dir, "content", "instructions.md"))
	if err != nil {
		return "", fmt.Errorf("read bundle identity card: %w", err)
	}
	skillsDir := filepath.Join(dir, "content", "skills")
	sources, err := os.ReadDir(skillsDir)
	if err != nil {
		return "", fmt.Errorf("read bundle skills: %w", err)
	}
	roots := make([]string, 0)
	for _, source := range sources {
		if !source.IsDir() {
			continue
		}
		skills, err := os.ReadDir(filepath.Join(skillsDir, source.Name()))
		if err != nil {
			return "", fmt.Errorf("read bundle source %s: %w", source.Name(), err)
		}
		for _, skill := range skills {
			if skill.IsDir() {
				roots = append(roots, filepath.Join(skillsDir, source.Name(), skill.Name()))
			}
		}
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("bundle %s selected no skills", dir)
	}
	sort.Strings(roots)
	pack, err := LoadSkillpack(roots)
	if err != nil {
		return "", err
	}
	body := stripFrontmatter(string(card))
	if body == "" {
		return "", fmt.Errorf("bundle identity card is empty")
	}
	return body + "\n\n" + pack, nil
}

func isSkillEntrypoint(slashed string) bool {
	for _, name := range skillEntrypoints {
		if slashed == name {
			return true
		}
	}
	return false
}

func stripFrontmatter(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return strings.TrimSpace(normalized)
	}
	remainder := normalized[4:]
	end := strings.Index(remainder, "\n---\n")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(remainder[end+5:])
}
