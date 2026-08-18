package community

import (
	"strings"
	"testing"
	"unicode"
)

// The decisions this pins are Kai's on sirens-echo#203: objects in, status
// indicators out, tone out, and at most three.

func TestNeutralStyleAdmitsAnObjectEmoji(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Wood 🪵 is listed at 3 Spectres.",
		"Iron Ore 🪨 is mined in the eastern hills.",
		"The recipe needs Wheat 🌾 and Water 💧.",
		"Elk 🦌 spawn in the boreal forest.",
	} {
		reply := reply
		t.Run(reply, func(t *testing.T) {
			t.Parallel()
			if err := ValidateNeutralStyle(reply); err != nil {
				t.Fatalf("an object emoji was refused: %v", err)
			}
		})
	}
}

// An indicator is legible and is not an object, which is the line Kai drew
// when she ruled the server-status dot out of scope.
func TestNeutralStyleRefusesAnIndicator(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"The Eco server is online 🟢.",
		"That trade is gone ❌.",
		"The build completed ✅.",
		"Stock is falling ⬇.",
	} {
		reply := reply
		t.Run(reply, func(t *testing.T) {
			t.Parallel()
			if err := ValidateNeutralStyle(reply); err == nil {
				t.Fatal("an indicator emoji was admitted")
			}
		})
	}
}

// Tone is what the neutral profile exists to keep out, and it is unchanged.
func TestNeutralStyleRefusesTone(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"The server is online 🙂.",
		"That trade is gone 😞.",
		"Stock is low 👀.",
		"Wood is plentiful 👍.",
		"The build finished 🎉.",
	} {
		reply := reply
		t.Run(reply, func(t *testing.T) {
			t.Parallel()
			if err := ValidateNeutralStyle(reply); err == nil {
				t.Fatal("an emotive emoji was admitted")
			}
		})
	}
}

// The bound is Kai's. One past it is what turns legibility into noise, whatever
// the bound is set to.
func TestNeutralStyleBoundsObjectEmoji(t *testing.T) {
	t.Parallel()
	stock := []string{"Wood 🪵", "Stone 🪨", "Wheat 🌾", "Water 💧", "Iron ⚙️", "Coal 🪨"}
	if len(stock) < maxObjectEmoji+1 {
		t.Fatalf("the fixture carries %d items and the bound is %d; add more", len(stock), maxObjectEmoji)
	}
	atBound := strings.Join(stock[:maxObjectEmoji], ", ") + " are stocked."
	if err := ValidateNeutralStyle(atBound); err != nil {
		t.Fatalf("%d object emoji were refused: %v", maxObjectEmoji, err)
	}
	overBound := strings.Join(stock[:maxObjectEmoji+1], ", ") + " are stocked."
	if err := ValidateNeutralStyle(overBound); err == nil {
		t.Fatalf("object emoji number %d was admitted", maxObjectEmoji+1)
	}
}

// A joined glyph is one emoji to a reader, so counting its runes would refuse
// a reply that shows three.
func TestAJoinedGlyphCountsOnce(t *testing.T) {
	t.Parallel()
	// A flag is a pair of regional indicators, and a rune count would read two.
	count, refused := objectEmojiCount("The map 🗺️ marks it.")
	if refused {
		t.Fatal("a map with a presentation selector was refused")
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

// The grave accent is ASCII and lives in Sk, so scanning that category without
// the ASCII guard bans a code span. Regression on 2ef1c16.
func TestACodeSpanSurvivesTheEmojiScan(t *testing.T) {
	t.Parallel()
	if err := ValidateNeutralStyle("The tool is `get_eco_server_status`."); err != nil {
		t.Fatalf("a code span was refused: %v", err)
	}
}

// The doctrine asks for the emoji after the object. The check must not forbid
// the shape the doctrine requires.
func TestTheCheckAdmitsTheDocumentedShape(t *testing.T) {
	t.Parallel()
	reply := "Wood 🪵 is listed at 3 Spectres."
	if !strings.Contains(reply, "Wood 🪵") {
		t.Fatal("fixture drifted from the documented shape")
	}
	if err := ValidateNeutralStyle(reply); err != nil {
		t.Fatalf("the documented shape was refused: %v", err)
	}
}

// unicode.Is switches from a linear scan to a binary search past eighteen
// entries, so an unsorted table would start missing runes silently.
func TestTheRefusedTablesStaySorted(t *testing.T) {
	t.Parallel()
	for name, table := range map[string]*unicode.RangeTable{
		"emotiveRunes":   emotiveRunes,
		"indicatorRunes": indicatorRunes,
	} {
		for i := 1; i < len(table.R16); i++ {
			if table.R16[i].Lo <= table.R16[i-1].Hi {
				t.Errorf("%s R16[%d] is out of order or overlapping", name, i)
			}
		}
		for i := 1; i < len(table.R32); i++ {
			if table.R32[i].Lo <= table.R32[i-1].Hi {
				t.Errorf("%s R32[%d] is out of order or overlapping", name, i)
			}
		}
	}
}
