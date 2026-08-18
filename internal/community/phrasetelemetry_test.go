package community

import "testing"

// Invocation counts per key were the third reason sirens-echo#176 gave for
// preferring a tool call. The sentinel can carry them too.
func TestInvokedKeysNamesEveryKeyInOrder(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		reply string
		want  []string
	}{
		{"{{phrase:no-tool}}", []string{"no-tool"}},
		{"{{phrase:no-tool}} {{phrase:no-data}}", []string{"no-tool", "no-data"}},
		{"an ordinary reply", nil},
		{"{{phrase: no-tool }}", []string{"no-tool"}},
	} {
		got := InvokedKeys(row.reply)
		if len(got) != len(row.want) {
			t.Errorf("InvokedKeys(%q) = %v, want %v", row.reply, got, row.want)
			continue
		}
		for i := range got {
			if got[i] != row.want[i] {
				t.Errorf("InvokedKeys(%q)[%d] = %q, want %q", row.reply, i, got[i], row.want[i])
			}
		}
	}
}

// The key is what a metric labels, so it must be the registry's spelling and
// not whatever whitespace the model wrote around it.
func TestAnInvokedKeyIsTrimmedForTheLabel(t *testing.T) {
	t.Parallel()
	keys := InvokedKeys("{{phrase:  no-tool  }}")
	if len(keys) != 1 || keys[0] != "no-tool" {
		t.Fatalf("keys = %v, want [no-tool]", keys)
	}
}

// An exact key beats a keyword list, which is the eval payoff the issue named.
func TestCheckExpectedPhraseScoresOnTheKey(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		reply string
		want  string
		ok    bool
	}{
		{"the expected key alone", "{{phrase:no-tool}}", "no-tool", true},
		{"a different key", "{{phrase:no-data}}", "no-tool", false},
		{"prose that reads like a refusal", "No tool for that.", "no-tool", false},
		{"the key beside prose", "Sorry. {{phrase:no-tool}}", "no-tool", false},
		{"two keys", "{{phrase:no-tool}}{{phrase:no-data}}", "no-tool", false},
	} {
		row := row
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := checkExpectedPhrase(row.reply, row.want)
			if row.ok && err != nil {
				t.Fatalf("expected pass: %v", err)
			}
			if !row.ok && err == nil {
				t.Fatal("expected failure")
			}
		})
	}
}

// A case naming only expect_phrase still scores something, or the pack loader
// would reject it as scoring nothing.
func TestExpectPhraseAloneCountsAsScored(t *testing.T) {
	t.Parallel()
	if !(EvaluationCase{ExpectPhrase: "no-tool"}).checked() {
		t.Fatal("a case scoring only on a phrase key reads as scoring nothing")
	}
}

// The eval built its prompt without the phrase policy, so it measured a prompt
// no deployment renders. With no registry configured both are byte-identical.
func TestTheEvaluationPromptCarriesThePhrasePolicy(t *testing.T) {
	t.Setenv("SIRENS_ECHO_PHRASES", "")
	definition := Definition{Identity: "Sirens Echo", ResponseStyle: ResponseStyleNeutral}
	built, err := evaluationSystemPrompt(definition, PlaceholderPrincipal, "", "policy")
	if err != nil {
		t.Fatalf("evaluationSystemPrompt: %v", err)
	}
	// The reaction policy is compiled in rather than configured, so it is part
	// of the baseline an unconfigured registry is measured against.
	plain := withReactionPolicy(
		BuildSystemPrompt(definition, PlaceholderPrincipal, "", "policy"))
	if built != plain {
		t.Error("an unconfigured registry changed the prompt, so every snapshot moves")
	}
}
