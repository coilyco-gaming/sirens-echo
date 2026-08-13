package community

import (
	"errors"
	"strings"
	"testing"
)

// The harness knows which check refused the reply. A production repair turn
// spent its whole budget deducing it and emitted nothing. See #549, #651.
func TestTheRepairPromptNamesTheCheckThatRefused(t *testing.T) {
	t.Parallel()
	for _, style := range []string{ResponseStyleNeutral, ResponseStyleSocial, ""} {
		prompt := responseRepairPrompt(style, errors.New("model reply exceeds 1800 characters"))
		if !strings.Contains(prompt, "model reply exceeds 1800 characters") {
			t.Errorf("style %q: the reason is absent, so the model must deduce it:\n%s",
				style, prompt)
		}
	}
}

// The instruction has to survive beside the reason, or naming the check trades
// one missing half for another.
func TestTheRepairPromptKeepsItsStyleInstruction(t *testing.T) {
	t.Parallel()
	refused := errors.New("model reply used an exclamation mark")
	neutral := responseRepairPrompt(ResponseStyleNeutral, refused)
	if !strings.Contains(neutral, "neutral, concise, impersonal language") {
		t.Errorf("the neutral instruction was lost:\n%s", neutral)
	}
	social := responseRepairPrompt(ResponseStyleSocial, refused)
	if !strings.Contains(social, "selected social tone") {
		t.Errorf("the social instruction was lost:\n%s", social)
	}
}

// A nil reason must not render the word nil at the model. The repair path
// always has one, so this guards a future caller rather than today's.
func TestARepairPromptWithNoReasonIsUnchanged(t *testing.T) {
	t.Parallel()
	if got := responseRepairPrompt(ResponseStyleNeutral, nil); got != neutralResponseRepairPrompt {
		t.Errorf("a nil reason changed the prompt:\n%s", got)
	}
	if got := responseRepairPrompt(ResponseStyleSocial, nil); got != socialResponseRepairPrompt {
		t.Errorf("a nil reason changed the prompt:\n%s", got)
	}
}

// Every reason the repair path can carry is a fixed sentence about the reply.
// If a validator ever interpolates member text, this is where it is noticed.
func TestEveryContractReasonIsFreeOfMemberText(t *testing.T) {
	t.Parallel()
	replies := []string{
		"", strings.Repeat("x", 1801),
		"Hello there! I'd love to help 🙂",
		"I think we should check the server.",
	}
	for _, style := range []string{ResponseStyleNeutral, ResponseStyleSocial} {
		for _, reply := range replies {
			parsed, err := ParseReply(reply)
			if err == nil {
				err = ValidateResponseStyle(style, parsed)
			}
			if err == nil {
				continue
			}
			if !strings.HasPrefix(err.Error(), "model ") {
				t.Errorf("reason %q does not describe the model's own reply", err)
			}
			if strings.Contains(err.Error(), reply) && reply != "" {
				t.Errorf("reason %q quotes the reply back into the prompt", err)
			}
		}
	}
}
