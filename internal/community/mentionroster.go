package community

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Naming someone should reach them. Only people already in the conversation
// can be reached. See docs/sirens-echo-mentions.md.

// mentionNameRunes is the shortest name worth resolving. A one or two
// character display name matches too much ordinary prose to be safe.
const mentionNameRunes = 3

// mentionRoster maps a display name to the account it belongs to, built from
// the turn's own transcript.
type mentionRoster map[string]string

// add records a name, ignoring one too short to match safely or one already
// known. First writer wins, so the oldest message in the turn decides.
func (r mentionRoster) add(name, userID string) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < mentionNameRunes || strings.TrimSpace(userID) == "" {
		return
	}
	if _, known := r[name]; !known {
		r[name] = userID
	}
}

// names returns the roster's names longest first, so a display name that
// contains another is matched before its substring.
func (r mentionRoster) names() []string {
	names := make([]string, 0, len(r))
	for name := range r {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) > len(names[j])
		}
		return names[i] < names[j]
	})
	return names
}

// mentionSpan is a run of the reply. A link is carried through byte-identical,
// because a display name matching a path component is not that person.
type mentionSpan struct {
	text string
	link bool
}

// mentionSpans splits a reply on the same urlSpan the prose validators mask
// with, so where a link starts has one definition. See sirens-echo#465.
func mentionSpans(reply string) []mentionSpan {
	spans := make([]mentionSpan, 0, 3)
	end := 0
	for _, bounds := range urlSpan.FindAllStringIndex(reply, -1) {
		if bounds[0] > end {
			spans = append(spans, mentionSpan{text: reply[end:bounds[0]]})
		}
		spans = append(spans, mentionSpan{text: reply[bounds[0]:bounds[1]], link: true})
		end = bounds[1]
	}
	if end < len(reply) {
		spans = append(spans, mentionSpan{text: reply[end:]})
	}
	return spans
}

// resolveMentions rewrites each named person as a Discord mention and reports
// the accounts it resolved. Nothing else in the reply is touched.
func (r mentionRoster) resolveMentions(reply string) (string, []string) {
	if len(r) == 0 || strings.TrimSpace(reply) == "" {
		return reply, nil
	}
	resolved := make([]string, 0, len(r))
	seen := make(map[string]bool, len(r))
	spans := mentionSpans(reply)
	for _, name := range r.names() {
		userID := r[name]
		if seen[userID] {
			continue
		}
		pattern, err := regexp.Compile(`(?i)(^|[^\w<@])` + regexp.QuoteMeta(name) + `\b`)
		if err != nil {
			continue
		}
		// Once per person, and prose only. A reply naming someone four times
		// should reach them once, not ping them four times.
		if !resolveWithin(spans, pattern, userID) {
			continue
		}
		seen[userID] = true
		resolved = append(resolved, userID)
	}
	var rewritten strings.Builder
	for _, span := range spans {
		rewritten.WriteString(span.text)
	}
	return rewritten.String(), resolved
}

// resolveWithin replaces the first prose occurrence in place and reports
// whether it found one. A match only inside a link reaches nobody.
func resolveWithin(spans []mentionSpan, pattern *regexp.Regexp, userID string) bool {
	for index := range spans {
		if spans[index].link {
			continue
		}
		text := spans[index].text
		// Every occurrence, not the first. The first can be a hostname label
		// while a real mention follows it. See sirens-echo#481.
		for _, where := range pattern.FindAllStringSubmatchIndex(text, -1) {
			if inDottedIdentifier(text, where[3], where[1]) {
				continue
			}
			// Cut on the match rather than on the roster's spelling of the
			// name, which the case-insensitive pattern need not match.
			spans[index].text = text[:where[3]] + "<@" + userID + ">" + text[where[1]:]
			return true
		}
	}
	return false
}

// inDottedIdentifier reports whether a name sits inside a dotted identifier
// such as a hostname, where it is a label and not a person.
func inDottedIdentifier(text string, start, end int) bool {
	if start > 0 && text[start-1] == '.' {
		return true
	}
	// A trailing dot before a letter or digit continues an identifier. A
	// trailing dot before a space or the end is the punctuation of a sentence.
	rest := text[end:]
	if !strings.HasPrefix(rest, ".") {
		return false
	}
	next, width := utf8.DecodeRuneInString(rest[1:])
	if width == 0 {
		return false
	}
	return unicode.IsLetter(next) || unicode.IsDigit(next)
}
