package community

import (
	"strings"
	"testing"
)

func blockPrincipal() Principal {
	return Principal{Handle: "coilysiren", UserID: "318190481467244544"}
}

// A sensitive class names nothing, because naming it says what to avoid saying
// next time. No model text reaches the member at all.
func TestASensitiveBlockCarriesNoModelText(t *testing.T) {
	t.Parallel()
	sensitive := ContentClass{ID: "minor-suspected", Deny: true, Sensitive: true}
	got := BlockResponse(sensitive, "The request suggests the member is a minor.", blockPrincipal())
	if got != blockRedirect {
		t.Fatalf("a sensitive block carried model text: %q", got)
	}
	if strings.Contains(got, "minor") {
		t.Fatal("the sensitive class leaked into the response")
	}
}

// An ordinary denied class may explain itself, briefly.
func TestAnOrdinaryBlockKeepsAShortReason(t *testing.T) {
	t.Parallel()
	denied := ContentClass{ID: "creative-long-form", Deny: true}
	reason := "Long-form fiction is outside this service's scope."
	if got := BlockResponse(denied, reason, blockPrincipal()); got != reason {
		t.Fatalf("reason = %q, want it kept", got)
	}
}

// Every reply guard the normal path runs, on text that would otherwise reach a
// member having passed none of them.
func TestAnUntrustworthyReasonFallsBackToTheRedirect(t *testing.T) {
	t.Parallel()
	denied := ContentClass{ID: "creative-long-form", Deny: true}
	for name, reason := range map[string]string{
		"echoes the principal id": "Only 318190481467244544 may request that.",
		"echoes the handle":       "Only coilysiren may request that.",
		"claims to be a person":   "I am a real person and cannot help with that.",
		"first person voice":      "I will not write that for you.",
		"too long": "This request falls outside the scope of the service because the " +
			"policy covering creative work excludes long-form fiction written on request " +
			"for members in this channel.",
		"empty":     "   ",
		"multiline": "\n\n",
	} {
		reason := reason
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := BlockResponse(denied, reason, blockPrincipal()); got != blockRedirect {
				t.Fatalf("an untrustworthy reason shipped: %q", got)
			}
		})
	}
}

// A block that fails open is the worst outcome available, so every fallback
// still refuses.
func TestEveryOutcomeIsStillARefusal(t *testing.T) {
	t.Parallel()
	for name, class := range map[string]ContentClass{
		"sensitive": {ID: "nsfw", Deny: true, Sensitive: true},
		"ordinary":  {ID: "creative-long-form", Deny: true},
	} {
		class := class
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := BlockResponse(class, "", blockPrincipal()); strings.TrimSpace(got) == "" {
				t.Fatal("a block produced no response at all")
			}
		})
	}
}

// A multi-line reason reads as an argument, and an argument invites the next
// message.
func TestABlockResponseIsOneLine(t *testing.T) {
	t.Parallel()
	denied := ContentClass{ID: "creative-long-form", Deny: true}
	got := BlockResponse(denied, "Out of scope.\nThe policy excludes it.", blockPrincipal())
	if strings.Contains(got, "\n") {
		t.Fatalf("block response spans lines: %q", got)
	}
}

// The response must itself survive the checks a reply survives, since it is a
// reply as far as the member is concerned.
func TestABlockResponsePassesTheReplyChecks(t *testing.T) {
	t.Parallel()
	guard := corpusIdentifierGuard(t)
	for _, class := range []ContentClass{
		{ID: "nsfw", Deny: true, Sensitive: true},
		{ID: "creative-long-form", Deny: true},
	} {
		got := BlockResponse(class, "Long-form fiction is outside scope.", blockPrincipal())
		if err := ValidateNeutralStyle(got); err != nil {
			t.Errorf("block response breaks neutral style: %v", err)
		}
		if err := ValidateIdentityClaim(got, blockPrincipal()); err != nil {
			t.Errorf("block response breaks the identity check: %v", err)
		}
		if err := guard.Validate(got); err != nil {
			t.Errorf("block response carried an identifier: %v", err)
		}
	}
}
