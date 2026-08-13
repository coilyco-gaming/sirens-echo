package community

import (
	"regexp"
	"sort"
	"strings"
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

// resolveMentions rewrites each named person as a Discord mention and reports
// the accounts it resolved. Nothing else in the reply is touched.
func (r mentionRoster) resolveMentions(reply string) (string, []string) {
	if len(r) == 0 || strings.TrimSpace(reply) == "" {
		return reply, nil
	}
	resolved := make([]string, 0, len(r))
	seen := make(map[string]bool, len(r))
	for _, name := range r.names() {
		userID := r[name]
		if seen[userID] {
			continue
		}
		pattern, err := regexp.Compile(`(?i)(^|[^\w<@])` + regexp.QuoteMeta(name) + `\b`)
		if err != nil {
			continue
		}
		if !pattern.MatchString(reply) {
			continue
		}
		// Once per person. A reply naming someone four times should reach them
		// once, not ping them four times.
		replaced := false
		reply = pattern.ReplaceAllStringFunc(reply, func(match string) string {
			if replaced {
				return match
			}
			replaced = true
			return strings.TrimSuffix(match, name) + "<@" + userID + ">"
		})
		seen[userID] = true
		resolved = append(resolved, userID)
	}
	return reply, resolved
}
