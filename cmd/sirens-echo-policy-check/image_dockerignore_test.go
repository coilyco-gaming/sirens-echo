package main_test

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// dockerignoreRule is one .dockerignore line. A negated rule re-includes what
// an earlier one excluded, and the last matching rule decides.
type dockerignoreRule struct {
	pattern string
	negated bool
}

func imageRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func loadDockerignore(t *testing.T, root string) []dockerignoreRule {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".dockerignore"))
	if err != nil {
		t.Fatalf("read .dockerignore: %v", err)
	}
	rules := make([]dockerignoreRule, 0, 8)
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule := dockerignoreRule{pattern: line}
		if strings.HasPrefix(line, "!") {
			rule = dockerignoreRule{pattern: strings.TrimPrefix(line, "!"), negated: true}
		}
		rules = append(rules, rule)
	}
	return rules
}

// ruleMatches tests the path and each ancestor, because excluding a directory
// excludes everything beneath it.
func ruleMatches(pattern, candidate string) bool {
	pattern = path.Clean(pattern)
	for current := path.Clean(candidate); ; current = path.Dir(current) {
		if matched, err := path.Match(pattern, current); err == nil && matched {
			return true
		}
		if current == "." || current == "/" {
			return false
		}
	}
}

func excludedFromContext(rules []dockerignoreRule, candidate string) bool {
	excluded := false
	for _, rule := range rules {
		if ruleMatches(rule.pattern, candidate) {
			excluded = !rule.negated
		}
	}
	return excluded
}

// dockerfileContextSources returns every COPY source read from the build
// context. A --from stage copy reads from an image and is not context.
func dockerfileContextSources(t *testing.T, root string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	sources := make([]string, 0, 16)
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "COPY ") {
			continue
		}
		fromStage := false
		operands := make([]string, 0, 4)
		for _, field := range strings.Fields(line)[1:] {
			if strings.HasPrefix(field, "--") {
				fromStage = fromStage || strings.HasPrefix(field, "--from=")
				continue
			}
			operands = append(operands, field)
		}
		if fromStage || len(operands) < 2 {
			continue
		}
		sources = append(sources, operands[:len(operands)-1]...)
	}
	return sources
}

// Both faults that turned main red were a Dockerfile and .dockerignore
// disagreeing. Neither needs a daemon to catch. See coilyco-gaming/sirens-echo#129.
func TestDockerfileCopiesOnlyPathsTheContextCarries(t *testing.T) {
	t.Parallel()
	root := imageRepoRoot(t)
	rules := loadDockerignore(t, root)
	sources := dockerfileContextSources(t, root)
	if len(sources) < 8 {
		t.Fatalf("parsed %d COPY sources, which is fewer than the Dockerfile has", len(sources))
	}
	for _, source := range sources {
		if _, err := os.Stat(filepath.Join(root, source)); err != nil {
			t.Errorf("Dockerfile copies %s but the repository has no such path", source)
			continue
		}
		if excludedFromContext(rules, source) {
			t.Errorf(
				".dockerignore excludes %s while the Dockerfile copies it, so that stage receives nothing",
				source,
			)
		}
	}
}

// The mirrored list is hand-maintained, so it can drift away from the COPY set
// it claims to mirror without anything noticing.
func TestImageContextPathsStillMirrorTheDockerfile(t *testing.T) {
	t.Parallel()
	root := imageRepoRoot(t)
	copied := make(map[string]struct{})
	for _, source := range dockerfileContextSources(t, root) {
		copied[filepath.Clean(source)] = struct{}{}
	}
	for _, mirrored := range imageContextPaths {
		if _, ok := copied[filepath.Clean(mirrored)]; !ok {
			t.Errorf("imageContextPaths names %s, which no Dockerfile COPY reads from the context", mirrored)
		}
	}
}

// The cases are the two regressions that reached main plus the negation that
// keeps the compose script reachable today.
func TestDockerignoreMatchingCoversTheFaultsThatShipped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		rules     []dockerignoreRule
		candidate string
		want      bool
	}{
		{
			"docs excluded while the build stage copies it",
			[]dockerignoreRule{{pattern: "docs"}},
			"docs",
			true,
		},
		{
			"the scripts glob hides the compose script",
			[]dockerignoreRule{{pattern: "scripts/*"}},
			"scripts/stage-compose-sources.sh",
			true,
		},
		{
			"the negation puts the compose script back",
			[]dockerignoreRule{
				{pattern: "scripts/*"},
				{pattern: "scripts/stage-compose-sources.sh", negated: true},
			},
			"scripts/stage-compose-sources.sh",
			false,
		},
		{
			"an excluded directory takes its children with it",
			[]dockerignoreRule{{pattern: "bin"}},
			"bin/sirens-echo",
			true,
		},
		{
			"an unrelated exclusion leaves the path alone",
			[]dockerignoreRule{{pattern: "assets"}},
			".agents/skills/coilyco-general",
			false,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := excludedFromContext(testCase.rules, testCase.candidate)
			if got != testCase.want {
				t.Errorf("excluded(%s) = %v, want %v", testCase.candidate, got, testCase.want)
			}
		})
	}
}
