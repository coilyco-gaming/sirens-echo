package community

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

// truncateRunes is the last gate before a message reaches Discord, called from
// the reply, command, and job paths. It had no test.

// A truncated value is exactly the limit, because the ellipsis replaces a rune
// rather than joining it. One rune over is what proves the boundary.
func TestTruncateRunesLandsExactlyOnTheLimit(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name  string
		value string
		limit int
		want  int
	}{
		{"under the limit", "short", 10, 5},
		{"exactly the limit", strings.Repeat("a", 10), 10, 10},
		{"one over", strings.Repeat("a", 11), 10, 10},
		{"far over", strings.Repeat("a", 500), 10, 10},
		{"empty", "", 5, 0},
	} {
		got := utf8.RuneCountInString(truncateRunes(testCase.value, testCase.limit))
		if got != testCase.want {
			t.Errorf("%s: %d runes, want %d", testCase.name, got, testCase.want)
		}
	}
}

// The cap counts runes, so a multibyte reply is not truncated for bytes it did
// not spend. This is the same property the turn contract holds for input.
func TestTruncateRunesCountsRunesNotBytes(t *testing.T) {
	t.Parallel()
	accented := truncateRunes(strings.Repeat("é", 20), 10)
	if got := utf8.RuneCountInString(accented); got != 10 {
		t.Errorf("accented: %d runes, want 10", got)
	}
	if len(accented) <= 10 {
		t.Errorf("accented: %d bytes, expected more than the rune count", len(accented))
	}
	emoji := truncateRunes(strings.Repeat("🙂", 20), 10)
	if got := utf8.RuneCountInString(emoji); got != 10 {
		t.Errorf("emoji: %d runes, want 10", got)
	}
}

// A limit of one or zero has no room for both a rune and the ellipsis, so it
// returns the prefix rather than a marker that would overrun.
func TestTruncateRunesHandlesTinyLimits(t *testing.T) {
	t.Parallel()
	if got := truncateRunes("abc", 1); got != "a" {
		t.Errorf("limit 1 = %q, want %q", got, "a")
	}
	if got := truncateRunes("abc", 0); got != "" {
		t.Errorf("limit 0 = %q, want empty", got)
	}
	if got := truncateRunes("abc", 2); utf8.RuneCountInString(got) != 2 {
		t.Errorf("limit 2 = %q, want 2 runes", got)
	}
}

// Characterization. A negative limit panics on the slice. No caller passes one
// today; a computed limit would find this at runtime, in the send path.
func TestTruncateRunesPanicsOnANegativeLimit(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("a negative limit no longer panics. If it was made total, " +
				"assert the value it returns instead")
		}
	}()
	truncateRunes("abc", -1)
}

// displayName prefers the guild nickname, then the global name, then the
// username, and never returns empty because it names the transcript author.
func TestDisplayNamePrefersTheMostSpecificName(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name    string
		message *discordgo.Message
		want    string
	}{
		{
			"nickname wins",
			&discordgo.Message{
				Member: &discordgo.Member{Nick: "Nick"},
				Author: &discordgo.User{GlobalName: "Global", Username: "user"},
			},
			"Nick",
		},
		{
			"blank nickname falls through",
			&discordgo.Message{
				Member: &discordgo.Member{Nick: "   "},
				Author: &discordgo.User{GlobalName: "Global", Username: "user"},
			},
			"Global",
		},
		{
			"blank global name falls through",
			&discordgo.Message{Author: &discordgo.User{GlobalName: " ", Username: "user"}},
			"user",
		},
		{"no author at all", &discordgo.Message{}, "member"},
	} {
		if got := displayName(testCase.message); got != testCase.want {
			t.Errorf("%s: %q, want %q", testCase.name, got, testCase.want)
		}
	}
}
