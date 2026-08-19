package community

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skillEntrypoints are the body filenames a policy root may use. Ordinary
// skills ship SKILL.md and agent-compose composed sources ship COMPOSED.md.
var skillEntrypoints = []string{"SKILL.md", "COMPOSED.md"}

// SkillReference is one reference file the model reads when it decides the file
// is relevant, rather than paying for it every turn. See sirens-echo#859.
type SkillReference struct {
	// Path is repo-relative and is the identifier read_skill takes.
	Path  string
	Title string
	Body  string
}

// LoadSkillpack loads what every turn carries: each root's entrypoint, plus the
// references that must not be optional. See docs/sirens-echo-prompt.md.
func LoadSkillpack(roots []string) (string, error) {
	pack, _, err := loadSkills(roots, nil)
	return pack, err
}

// LoadSkillReferences returns what read_skill serves, which is everything the
// pack left out.
func LoadSkillReferences(roots []string) ([]SkillReference, error) {
	_, references, err := loadSkills(roots, nil)
	return references, err
}

// loadSkills partitions the roots once, so the pack and the catalog cannot
// disagree about which file is in which.
func loadSkills(roots []string, eager map[string]bool) (string, []SkillReference, error) {
	files, err := skillFiles(roots)
	if err != nil {
		return "", nil, err
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("skillpack contains no SKILL.md or reference Markdown")
	}
	var output strings.Builder
	references := make([]SkillReference, 0)
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", nil, fmt.Errorf("read skill file %s: %w", path, err)
		}
		text := string(raw)
		body := stripFrontmatter(text)
		if body == "" {
			return "", nil, fmt.Errorf("skill file %s has no body", path)
		}
		slashed := filepath.ToSlash(path)
		// The contract, restored. A description is the cheap always-on index and
		// the body is what gets fetched, for an entrypoint as for a reference.
		if !inlineAlways(text) && !eager[rootOf(path, eager)] {
			references = append(references, SkillReference{
				Path: slashed, Title: skillSummary(text, body), Body: body,
			})
			continue
		}
		fmt.Fprintf(&output, "\n## Source: %s\n\n%s\n", slashed, body)
		if output.Len() > maxSkillpackBytes {
			return "", nil, fmt.Errorf("skillpack exceeds %d bytes", maxSkillpackBytes)
		}
	}
	return strings.TrimSpace(output.String() + skillIndex(references)), references, nil
}

// rootOf reports which eager root a path sits under, so a file inherits its
// root's eagerness without every caller threading the root down to it.
func rootOf(path string, eager map[string]bool) string {
	for root := range eager {
		if strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/") {
			return root
		}
	}
	return ""
}

// skillFiles lists every entrypoint and one-level reference, in deterministic
// order, across every root.
func skillFiles(roots []string) ([]string, error) {
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
			if isSkillEntrypoint(filepath.ToSlash(relative)) || isReferencePath(path) {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walk skill root %s: %w", root, err)
		}
	}
	sort.Strings(files)
	return files, nil
}

// isReferencePath reports a one-level reference file under a skill root.
func isReferencePath(path string) bool {
	slashed := filepath.ToSlash(path)
	return strings.Contains(slashed, "/references/") && strings.HasSuffix(slashed, ".md")
}

// inlineAlways reports a reference that must not be optional. A model that has
// to choose to read its own boundaries may not. See docs/sirens-echo-prompt.md.
func inlineAlways(raw string) bool {
	return frontmatterFlag(raw, "inline")
}

// skillIndex tells the model what it can read, because a file it cannot see is
// a file it will not ask for.
func skillIndex(references []SkillReference) string {
	if len(references) == 0 {
		return ""
	}
	var index strings.Builder
	index.WriteString("\n## Readable references\n\n" +
		"These are not in this prompt. Call read_skill with the path to read one, " +
		"and do it before answering from memory on the subject it names.\n\n")
	for _, reference := range references {
		fmt.Fprintf(&index, "- `%s` - %s\n", reference.Path, reference.Title)
	}
	return index.String()
}

// skillSummary is what the index says a file is about. A skill declares that in
// its own frontmatter, so scraping a heading is the fallback rather than first.
func skillSummary(raw, body string) string {
	if description := frontmatterValue(raw, "description"); description != "" {
		return description
	}
	return firstHeading(body)
}

// firstHeading is what the index says a reference is about.
func firstHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimLeft(trimmed, "# "))
		}
	}
	return "reference material"
}

// PlaceholderComposed keeps the tracked snapshot and the build-time policy
// check hermetic. It carries the surface the validator anchors on.
const PlaceholderComposed = `# Role instructions

Agent-compose assigned you the ` + "`<role>`" + ` role from the caller's compose request.

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

// rosterSource holds the composer's own identity doctrine, which stays eager
// whatever those files declare. See docs/sirens-echo-compose.md.
const rosterSource = "roster:core"

// bundleRoots lists a bundle's skill roots. Shared so the pack and the
// references it defers come from one list rather than two walks that can drift.
func bundleRoots(dir string) ([]string, map[string]bool, error) {
	skillsDir := filepath.Join(dir, "content", "skills")
	sources, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read bundle skills: %w", err)
	}
	roots := make([]string, 0)
	eager := make(map[string]bool)
	for _, source := range sources {
		if !source.IsDir() {
			continue
		}
		// The directory is the escaped source name, so unescape rather than
		// matching the escaping, which is the encoder's business and not ours.
		name, unescapeErr := url.PathUnescape(source.Name())
		if unescapeErr != nil {
			name = source.Name()
		}
		skills, err := os.ReadDir(filepath.Join(skillsDir, source.Name()))
		if err != nil {
			return nil, nil, fmt.Errorf("read bundle source %s: %w", source.Name(), err)
		}
		for _, skill := range skills {
			if !skill.IsDir() {
				continue
			}
			root := filepath.Join(skillsDir, source.Name(), skill.Name())
			roots = append(roots, root)
			if name == rosterSource {
				eager[root] = true
			}
		}
	}
	if len(roots) == 0 {
		return nil, nil, fmt.Errorf("bundle %s selected no skills", dir)
	}
	sort.Strings(roots)
	return roots, eager, nil
}

// LoadBundle reads one materialized agent-compose bundle: the identity card
// plus every selected skill as its own policy root. See docs/sirens-echo-compose.md.
func LoadBundle(dir string) (string, error) {
	card, err := os.ReadFile(filepath.Join(dir, "content", "instructions.md"))
	if err != nil {
		return "", fmt.Errorf("read bundle identity card: %w", err)
	}
	roots, eager, err := bundleRoots(dir)
	if err != nil {
		return "", err
	}
	pack, _, err := loadSkills(roots, eager)
	if err != nil {
		return "", err
	}
	body := stripFrontmatter(string(card))
	if body == "" {
		return "", fmt.Errorf("bundle identity card is empty")
	}
	return body + "\n\n" + pack, nil
}

// LoadBundleReferences serves what LoadBundle's pack left out, whose index
// names these paths, so read_skill has to hold them. See sirens-echo#859.
func LoadBundleReferences(dir string) ([]SkillReference, error) {
	roots, eager, err := bundleRoots(dir)
	if err != nil {
		return nil, err
	}
	_, references, err := loadSkills(roots, eager)
	return references, err
}

func isSkillEntrypoint(slashed string) bool {
	for _, name := range skillEntrypoints {
		if slashed == name {
			return true
		}
	}
	return false
}

// frontmatterValue reads one string key out of a file's frontmatter. Returns
// empty for an absent key or an absent block, so a caller falls back.
func frontmatterValue(raw, key string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return ""
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return ""
	}
	for _, line := range strings.Split(normalized[4:4+end], "\n") {
		name, value, found := strings.Cut(line, ":")
		if found && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// frontmatterFlag reads one boolean key out of a file's frontmatter, so the
// decision about a reference lives in the reference.
func frontmatterFlag(raw, key string) bool {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return false
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return false
	}
	for _, line := range strings.Split(normalized[4:4+end], "\n") {
		name, value, found := strings.Cut(line, ":")
		if found && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value) == "always" || strings.TrimSpace(value) == "true"
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
