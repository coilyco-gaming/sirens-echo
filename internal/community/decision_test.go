package community

import (
	"strings"
	"testing"
)

func TestParseReplyAcceptsPlainProse(t *testing.T) {
	t.Parallel()
	reply, err := ParseReply("  Welcome to the community.  ")
	if err != nil {
		t.Fatalf("ParseReply: %v", err)
	}
	if reply != "Welcome to the community." {
		t.Fatalf("reply = %q", reply)
	}
}

// A fenced block is legitimate reply content now that the envelope is gone.
// The old parser unwrapped fences and JSON, which would corrupt this answer.
func TestParseReplyPreservesFencedAndBracedContent(t *testing.T) {
	t.Parallel()
	raw := "Run this:\n```sh\necho {\"reply\":\"x\"}\n```"
	reply, err := ParseReply(raw)
	if err != nil {
		t.Fatalf("ParseReply: %v", err)
	}
	if reply != raw {
		t.Fatalf("reply = %q, want the input verbatim", reply)
	}
}

func TestParseReplyRejectsEmptyAndOverlongReplies(t *testing.T) {
	t.Parallel()
	if _, err := ParseReply("   "); err == nil {
		t.Fatal("expected empty reply error")
	}
	if _, err := ParseReply(strings.Repeat("a", 1801)); err == nil {
		t.Fatal("expected overlong reply error")
	}
}

func TestValidateGroundingRejectsInventedChannel(t *testing.T) {
	t.Parallel()
	reply := "Please check #events."
	err := ValidateGrounding(reply, "The current channel is #bots.")
	if err == nil || !strings.Contains(err.Error(), "invented channel") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateGroundingRejectsClaimedAction(t *testing.T) {
	t.Parallel()
	reply := "I opened an issue for this."
	err := ValidateGrounding(reply, "The current channel is #bots.")
	if err == nil || !strings.Contains(err.Error(), "claimed an action") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateGroundingAllowsClaimConfirmedByTool(t *testing.T) {
	t.Parallel()
	reply := "I closed issue 42."
	err := ValidateGrounding(
		reply,
		"The current channel is #bots.",
		ExecutedTool{Name: "forgejo__close_issue"},
	)
	if err != nil {
		t.Fatalf("ValidateGrounding: %v", err)
	}
}

func TestValidateGroundingRejectsUnsupportedClaimAfterConfirmedTool(t *testing.T) {
	t.Parallel()
	reply := "I closed issue 42, then I deleted its comments."
	err := ValidateGrounding(
		reply,
		"The current channel is #bots.",
		ExecutedTool{Name: "forgejo__close_issue"},
	)
	if err == nil || !strings.Contains(err.Error(), "claimed an action") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateGroundingAllowsSuppliedChannel(t *testing.T) {
	t.Parallel()
	reply := "This is the #bots sandbox."
	if err := ValidateGrounding(reply, "The current channel is #bots."); err != nil {
		t.Fatalf("ValidateGrounding: %v", err)
	}
}

func TestValidateGroundingAllowsForgejoIssueReference(t *testing.T) {
	t.Parallel()
	reply := "The latest open Forgejo issue is #57."
	if err := ValidateGrounding(reply, "The current channel is #bots."); err != nil {
		t.Fatalf("ValidateGrounding: %v", err)
	}
}

func TestValidateNeutralStyleRejectsPersonalityTraits(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"Hey there! 🫡",
		"I'm Sirens Echo, your automated community host.",
		"So here's the thing: the requested tool is unavailable.",
		"The request cannot be completed. Would you like a draft instead?",
		"The Eco server is online!",
		"The Eco server is online 🟢.",
		"Thanks for flagging that.",
	} {
		reply := reply
		t.Run(reply, func(t *testing.T) {
			t.Parallel()
			if err := ValidateNeutralStyle(reply); err == nil {
				t.Fatal("expected personality trait rejection")
			}
		})
	}
}

func TestValidateNeutralStyleAcceptsDirectResults(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"The Eco server is online.",
		"The earlier answer is unverified pending source review.",
		"No approved event time is available.",
	} {
		if err := ValidateNeutralStyle(reply); err != nil {
			t.Errorf("ValidateNeutralStyle(%q): %v", reply, err)
		}
	}
}

func TestValidateResponseStyleAllowsSocialVoice(t *testing.T) {
	t.Parallel()
	reply := "Hey! I found the Eco server status for you."
	if err := ValidateResponseStyle(ResponseStyleSocial, reply); err != nil {
		t.Fatalf("ValidateResponseStyle: %v", err)
	}
	if err := ValidateResponseStyle(ResponseStyleNeutral, reply); err == nil {
		t.Fatal("neutral response style accepted social voice")
	}
}

// A link is not prose: `coilysiren.me` ends in the pronoun "me" and a Forgejo
// fragment looks like a channel.
func TestValidateNeutralStyleAllowsLinks(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		"The tracked report is https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233",
		"Trade history is at https://eco.coilysiren.me/trades",
		"The reference is https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233#issue-8117",
	} {
		reply := reply
		t.Run(reply, func(t *testing.T) {
			t.Parallel()
			if err := ValidateNeutralStyle(reply); err != nil {
				t.Fatalf("ValidateNeutralStyle rejected a link: %v", err)
			}
		})
	}
}

func TestValidateGroundingAllowsURLFragment(t *testing.T) {
	t.Parallel()
	reply := "The report is https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233#issue-8117"
	if err := ValidateGrounding(reply, "The current channel is #bots."); err != nil {
		t.Fatalf("ValidateGrounding rejected a URL fragment: %v", err)
	}
}

// Masking must not weaken the check for prose outside the link.
func TestValidateNeutralStyleStillRejectsProseAroundLinks(t *testing.T) {
	t.Parallel()
	reply := "Here is my tracked report: https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233"
	if err := ValidateNeutralStyle(reply); err == nil {
		t.Fatal("ValidateNeutralStyle accepted first-person prose beside a link")
	}
}

func TestValidateGroundingStillRejectsInventedChannelBesideLink(t *testing.T) {
	t.Parallel()
	reply := "See #announcements and https://forgejo.coilysiren.me/coilyco-gaming/sirens-echo/issues/233"
	if err := ValidateGrounding(reply, "The current channel is #bots."); err == nil {
		t.Fatal("ValidateGrounding accepted an invented channel beside a link")
	}
}
