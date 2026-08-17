package community

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// A member who replies to a notice and says "trace" has named the id by
// pointing at it. See sirens-echo#339.

const sampleTraceID = "3dd883c6becba130e9f8b75e4593a94d"

func TestALookupNeedsBothTheWordAndAnID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		message    string
		referenced string
		want       bool
		quoted     bool
	}{
		{
			name:    "the id typed out with the word",
			message: "trace " + sampleTraceID,
			want:    true,
		},
		{
			name:       "replying to the notice that carried it",
			message:    "what happened here, trace please",
			referenced: "> `turn timed out, retry shortly`\n> `trace id " + sampleTraceID + "`",
			want:       true,
			quoted:     true,
		},
		{
			// The word alone cannot name a turn, and guessing which one would
			// be worse than not answering.
			name:    "the word with nothing to resolve",
			message: "can you trace that for me",
		},
		{
			// A member quoting a hash is not asking for telemetry, and this is
			// the false positive the keyword exists to prevent.
			name:    "an id with no ask",
			message: "the build is " + sampleTraceID,
		},
		{
			name:    "a longer hex run is not a trace id",
			message: "trace " + sampleTraceID + "cafe",
		},
		{
			name:    "retrace is not trace",
			message: "let me retrace my steps through " + sampleTraceID,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			lookup, asked := detectTraceLookup(testCase.message, testCase.referenced)
			if asked != testCase.want {
				t.Fatalf("asked = %v, want %v (lookup %+v)", asked, testCase.want, lookup)
			}
			if !testCase.want {
				return
			}
			if lookup.TraceID != sampleTraceID {
				t.Errorf("trace id = %q, want %q", lookup.TraceID, sampleTraceID)
			}
			if lookup.Quoted != testCase.quoted {
				t.Errorf("quoted = %v, want %v", lookup.Quoted, testCase.quoted)
			}
		})
	}
}

// What the member typed wins, because a member who types an id has named a
// different turn from the one they happen to be replying under.
func TestTheTypedIDBeatsTheQuotedOne(t *testing.T) {
	t.Parallel()
	other := "0123456789abcdef0123456789abcdef"
	lookup, asked := detectTraceLookup(
		"trace "+other,
		"> `trace id "+sampleTraceID+"`",
	)
	if !asked {
		t.Fatal("no lookup detected")
	}
	if lookup.TraceID != other {
		t.Errorf("trace id = %q, want the typed one %q", lookup.TraceID, other)
	}
	if lookup.Quoted {
		t.Error("a typed id reported itself as quoted")
	}
}

// Case is not the member's problem. The id is lowercase hex on the wire and a
// member pasting it from a console may not preserve that.
func TestAnUppercaseIDStillResolves(t *testing.T) {
	t.Parallel()
	lookup, asked := detectTraceLookup("TRACE 3DD883C6BECBA130E9F8B75E4593A94D", "")
	if !asked {
		t.Fatal("an uppercase id did not resolve")
	}
	if lookup.TraceID != sampleTraceID {
		t.Errorf("trace id = %q, want the lowercase form", lookup.TraceID)
	}
}

func TestTheDiscordTurnReadsTheReferencedMessage(t *testing.T) {
	t.Parallel()
	turn := &discordMessageTurn{message: &discordgo.Message{
		Content: "trace",
		ReferencedMessage: &discordgo.Message{
			Content: "> `turn failed`\n> `trace id " + sampleTraceID + "`",
		},
	}}
	lookup, asked := turn.TraceLookup()
	if !asked || lookup.TraceID != sampleTraceID || !lookup.Quoted {
		t.Fatalf("lookup = %+v, asked = %v", lookup, asked)
	}
	// A turn with no reference is the ordinary case and must not panic on the
	// nil the gateway leaves there.
	bare := &discordMessageTurn{message: &discordgo.Message{Content: "trace"}}
	if _, asked := bare.TraceLookup(); asked {
		t.Error("a turn with no reference and no id resolved a lookup")
	}
}

// The two have to keep agreeing, so a notice shape change fails here rather
// than quietly in production. See docs/sirens-echo-rate.md.
func TestTheHarnessNoticeIsStillReadableByThisDetector(t *testing.T) {
	t.Parallel()
	notice := harnessNotice("trace id " + sampleTraceID)
	lookup, asked := detectTraceLookup("trace?", notice)
	if !asked {
		t.Fatalf("the detector cannot read the harness's own notice: %q", notice)
	}
	if lookup.TraceID != sampleTraceID {
		t.Errorf("trace id = %q", lookup.TraceID)
	}
}
