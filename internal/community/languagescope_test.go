package community

import "testing"

// The reply guards match English words. Issue 298 binds a channel to a
// language, which makes the reach of those guards a shipping question.

// languagePair is one violation written twice. A guard catching one half and
// missing the other is scoped to a language rather than to the rule.
type languagePair struct {
	violation string
	english   string
	other     string
	// otherCaughtNow is the behavior on origin/main. The test asserts this, so
	// CI reports what ships rather than what is wanted.
	otherCaughtNow bool
	// otherShouldBeCaught is the intended behavior once a language is
	// configurable. Where the two differ the row names the issue.
	otherShouldBeCaught bool
	issue               string
}

// French because it is the example in issue 298. The point is not French: it is
// that every semantic rule below is keyed to English vocabulary.
var languagePairs = []languagePair{
	{
		violation:      "first-person voice",
		english:        "I am checking the server right away.",
		other:          "Je vérifie le serveur tout de suite.",
		otherCaughtNow: false, otherShouldBeCaught: true,
		issue: "298",
	},
	{
		violation:      "collective voice",
		english:        "We are looking into that for you.",
		other:          "Nous examinons cela pour vous.",
		otherCaughtNow: false, otherShouldBeCaught: true,
		issue: "298",
	},
	{
		violation:      "social opening",
		english:        "Hey there, that is a good question.",
		other:          "Salut, c'est une bonne question.",
		otherCaughtNow: false, otherShouldBeCaught: true,
		issue: "298",
	},
	{
		violation:      "work continuing past the turn",
		english:        "Sirens Echo is now monitoring the server.",
		other:          "Sirens Echo surveille désormais le serveur.",
		otherCaughtNow: false, otherShouldBeCaught: true,
		issue: "298",
	},
	{
		violation:      "conversational personality",
		english:        "Happy to help with that.",
		other:          "Ravi de pouvoir aider avec ça.",
		otherCaughtNow: false, otherShouldBeCaught: true,
		issue: "298",
	},
}

// caught reports whether any reply-path guard refuses this text.
func caught(reply string) bool {
	return ValidateNeutralStyle(reply) != nil || ValidateGrounding(reply, "") != nil
}

// The English half must keep firing. If it stops, the row below is measuring
// nothing and the miss would read as parity.
func TestEveryLanguagePairIsCaughtInEnglish(t *testing.T) {
	t.Parallel()
	if len(languagePairs) == 0 {
		t.Fatal("no language pairs, so the scope test asserts nothing")
	}
	for _, pair := range languagePairs {
		if !caught(pair.english) {
			t.Errorf("%s: the English half is no longer caught, so its pair proves nothing: %q",
				pair.violation, pair.english)
		}
	}
}

// Characterization. Every semantic guard is English-keyed, so the same
// violation in another language reaches the member unrefused.
func TestGuardScopeIsBoundToEnglish(t *testing.T) {
	t.Parallel()
	for _, pair := range languagePairs {
		got := caught(pair.other)
		if got == pair.otherCaughtNow {
			continue
		}
		if !pair.otherCaughtNow {
			t.Errorf("%s is now caught in the second language. If issue %s was "+
				"delivered, set otherCaughtNow to true and clear the issue field",
				pair.violation, pair.issue)
			continue
		}
		t.Errorf("regression: %s is no longer caught in the second language: %q",
			pair.violation, pair.other)
	}
}

// The two rules that do survive translation are character rules, not word
// rules. They are why a casual French spot-check looks like the guards work.
func TestOnlyCharacterRulesSurviveTranslation(t *testing.T) {
	t.Parallel()
	// Natural French ends an exclamation with a space before the mark, so a
	// realistic sample trips this rule and hides that the word rules missed.
	confounded := "Salut ! Ravi de pouvoir vous aider avec ça."
	if !caught(confounded) {
		t.Fatal("the exclamation rule stopped firing, so it no longer masks the word rules")
	}
	// The same sentence without the mark is the honest measurement.
	if caught("Salut, ravi de pouvoir vous aider avec ça.") {
		t.Error("a word rule fired in French; update the characterization rows above")
	}
	if !caught("Le serveur est en ligne 🎉") {
		t.Error("the decorative-symbol scan stopped firing on non-English text")
	}
}
