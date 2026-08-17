package community

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 447 and e5c9163 stopped the two code surfaces drifting apart. The doc is a
// third copy and nothing related it to either. See sirens-echo#447.

const glyphDoc = "sirens-echo-tool-markup.md"

// The table a reader consults has to name the glyphs the footer emits, or the
// documentation is a fourth spelling of a vocabulary that has already drifted.
func TestTheDocumentedGlyphsAreTheOnesTheFooterEmits(t *testing.T) {
	t.Parallel()
	documented := documentedStatusGlyphs(t)
	emitted := []string{toolOKGlyph, toolEmptyGlyph, toolFailedGlyph}
	sort.Strings(emitted)
	if strings.Join(documented, " ") != strings.Join(emitted, " ") {
		t.Errorf("docs/%s documents the status glyphs %v and the footer emits %v. "+
			"A reader checking the table against a reply sees a different symbol",
			glyphDoc, documented, emitted)
	}
}

// The hammer is not in the status table, it prefixes every line, so it is
// checked against the example block rather than against the three states.
func TestTheDocumentedPrefixIsTheOneTheFooterEmits(t *testing.T) {
	t.Parallel()
	body := glyphDocBody(t)
	if !strings.Contains(body, "> "+toolDisclosureGlyph+" ") {
		t.Errorf("docs/%s shows no example line prefixed with %q, which is what "+
			"every footer line opens with", glyphDoc, toolDisclosureGlyph)
	}
}

// documentedStatusGlyphs reads the first column of the glyph table, sorted.
func documentedStatusGlyphs(t *testing.T) []string {
	t.Helper()
	glyphs := make([]string, 0, 3)
	inTable := false
	for _, line := range strings.Split(glyphDocBody(t), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| Glyph ") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		glyph := strings.TrimSpace(cells[0])
		if glyph == "" || strings.HasPrefix(glyph, "---") {
			continue
		}
		glyphs = append(glyphs, glyph)
	}
	if len(glyphs) == 0 {
		t.Fatalf("found no glyph table in docs/%s, so this test covers nothing", glyphDoc)
	}
	sort.Strings(glyphs)
	return glyphs
}

func glyphDocBody(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", glyphDoc))
	if err != nil {
		t.Fatalf("read docs/%s: %v", glyphDoc, err)
	}
	return string(body)
}
