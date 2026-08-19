package community

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The drawers work only while they stay thin: the description is the always-on
// trigger, the body a fetched two sentences, and this pins that shape.

const (
	scienceDrawerGlob    = "../../.agents/skills/sirens-echo-science/references/*.md"
	maxDrawerBodyBytes   = 350
	maxDrawerDescription = 220
	minScienceDrawers    = 30
)

var drawerName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*\.md$`)

func TestScienceDrawersStayThin(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(scienceDrawerGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < minScienceDrawers {
		t.Fatalf("%d drawers, want at least %d: the catalogue is the feature", len(paths), minScienceDrawers)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		name := filepath.Base(path)
		if !drawerName.MatchString(name) {
			t.Errorf("%s: drawer names are lowercase-hyphen, they render on the worklog", name)
		}
		description := frontmatterValue(text, "description")
		if description == "" {
			t.Errorf("%s: no description, so the index has no trigger keywords", name)
		}
		if len(description) > maxDrawerDescription {
			t.Errorf("%s: description %d bytes exceeds %d", name, len(description), maxDrawerDescription)
		}
		if inlineAlways(text) {
			t.Errorf("%s: a drawer must stay a reference, inline defeats the design", name)
		}
		body := strings.TrimSpace(stripFrontmatter(text))
		if len(body) > maxDrawerBodyBytes {
			t.Errorf("%s: body %d bytes exceeds %d, two short sentences is the contract", name, len(body), maxDrawerBodyBytes)
		}
	}
}
