package community

import "testing"

// A phrase is the whole reply or it is not a phrase, and two phrases is not
// one. See sirens-echo#613.

func TestOneInvocationIsTerminal(t *testing.T) {
	t.Parallel()
	for name, reply := range map[string]string{
		"bare":               "{{phrase:no-tool}}",
		"leading space":      "  {{phrase:no-tool}}",
		"trailing newline":   "{{phrase:no-tool}}\n",
		"surrounded by both": "\n  {{phrase:no-tool}}  \n",
	} {
		if !Terminal(reply) {
			t.Errorf("%s is one invocation and was not terminal: %q", name, reply)
		}
	}
}

// The defect: stripping every invocation also leaves nothing, so several read
// as terminal and the member received the concatenation.
func TestSeveralInvocationsAreNotTerminal(t *testing.T) {
	t.Parallel()
	for name, reply := range map[string]string{
		"two adjacent":   "{{phrase:no-tool}}{{phrase:no-data}}",
		"two spaced":     "{{phrase:no-tool}} {{phrase:no-data}}",
		"the same twice": "{{phrase:no-tool}}{{phrase:no-tool}}",
		"three":          "{{phrase:a}}{{phrase:b}}{{phrase:c}}",
		"across lines":   "{{phrase:no-tool}}\n{{phrase:no-data}}",
	} {
		if Terminal(reply) {
			t.Errorf("%s was accepted as one phrase: %q", name, reply)
		}
	}
}

// A prefix stays non-terminal, which is what this function was built for and
// must not be traded away by counting.
func TestProseBesideAnInvocationIsNotTerminal(t *testing.T) {
	t.Parallel()
	for name, reply := range map[string]string{
		"prefix":  "Sorry, {{phrase:no-tool}}",
		"suffix":  "{{phrase:no-tool}} let me know if that helps",
		"both":    "Well, {{phrase:no-tool}} anyway",
		"no dots": "{{phrase:no-tool}} x",
	} {
		if Terminal(reply) {
			t.Errorf("%s carried prose and was terminal: %q", name, reply)
		}
	}
}

// A reply with no invocation at all is not terminal, or an ordinary reply
// would be treated as a phrase.
func TestAnOrdinaryReplyIsNotTerminal(t *testing.T) {
	t.Parallel()
	for name, reply := range map[string]string{
		"prose":        "The Eco server is online.",
		"empty":        "",
		"only spaces":  "   ",
		"braces alone": "{{}}",
	} {
		if Terminal(reply) {
			t.Errorf("%s was treated as a phrase: %q", name, reply)
		}
	}
}
