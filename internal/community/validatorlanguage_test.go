package community

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Which validators read words and which read values decides what still holds
// when a channel answers in another language. See docs/sirens-echo-language.md.

type validatorReach string

const (
	// reachSurvives matches a value, a delimiter, or a configured name, so a
	// translated reply is checked the same way an English one is.
	reachSurvives validatorReach = "survives"
	// reachMixed keeps one half of its guarantee across a translation.
	reachMixed validatorReach = "mixed"
	// reachLapses enumerates English surface forms and checks nothing else.
	reachLapses validatorReach = "lapses"
)

// validatorLanguageReach is recorded by hand. A heuristic over pattern text
// misclassified five of sixteen evaluation cases when I tried it there.
var validatorLanguageReach = map[string]validatorReach{
	// Delimiters and values. These are the security-relevant half.
	"ValidateNoToolCallMarkup": reachSurvives,
	"checkHandleEcho":          reachSurvives,
	// The numeric and encoded readings survive. spelledToDigits reads English
	// number words, so only that evasion goes unchecked.
	"checkUserIDEcho": reachMixed,
	// The invented-channel half matches a #token and survives. Every action
	// claim below it is an English verb list.
	"ValidateGrounding": reachMixed,
	// The emoji and exclamation halves are language independent. The three word
	// lists that carry the neutral profile are not.
	"ValidateNeutralStyle":  reachMixed,
	"ValidateResponseStyle": reachMixed,
	// The identity is quoted into the pattern and survives, but it is reached
	// only through an English copula and an English verb.
	"ValidateSelfAttributedClaim": reachLapses,
	"ValidateIdentityClaim":       reachLapses,
}

// A validator with no recorded reach is one nobody decided about, and the
// decision is what a channel in another language actually gets.
func TestEveryReplyValidatorRecordsItsLanguageReach(t *testing.T) {
	t.Parallel()
	declared := make(map[string]struct{}, len(validatorLanguageReach))
	names := regexp.MustCompile(`(?m)^func (Validate[A-Za-z]+|check(?:Handle|UserID)Echo)\(`)
	for _, file := range []string{"decision.go", "evaluation_checks.go"} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, match := range names.FindAllStringSubmatch(string(body), -1) {
			name := match[1]
			// Prompt validators score the prompt rather than a reply, and a
			// prompt is assembled here rather than written by a model.
			if strings.Contains(name, "SystemPrompt") {
				continue
			}
			declared[name] = struct{}{}
			if _, recorded := validatorLanguageReach[name]; !recorded {
				t.Errorf("%s has no recorded language reach. Decide whether its "+
					"guarantee survives a translation and add it to validatorLanguageReach",
					name)
			}
		}
	}
	stale := make([]string, 0)
	for name := range validatorLanguageReach {
		if _, live := declared[name]; !live {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("validatorLanguageReach records %s, which no longer exists",
			strings.Join(stale, ", "))
	}
}

// The map is an annotation until something proves it. These are the surviving
// halves, each run against a reply carrying the defect in French.
func TestTheValueAnchoredValidatorsStillFireInAnotherLanguage(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		name  string
		check func() error
	}{{
		name: "tool-call markup is a delimiter",
		check: func() error {
			return ValidateNoToolCallMarkup(
				"Voici le résultat <|tool_call|> pour votre demande.")
		},
	}, {
		name: "the operator handle is a value",
		check: func() error {
			return checkHandleEcho(
				"Le compte example_handle est celui du gestionnaire.", PlaceholderPrincipal)
		},
	}, {
		name: "the operator user ID is a value",
		check: func() error {
			return checkUserIDEcho(
				"L'identifiant configuré est 1024000000000000001 pour ce service.",
				PlaceholderPrincipal)
		},
	}, {
		name: "an invented channel is a token",
		check: func() error {
			return ValidateGrounding(
				"Consultez #salon-invente pour la suite.", "The current channel is #bots.")
		},
	}, {
		name: "an exclamation mark is punctuation",
		check: func() error {
			return ValidateNeutralStyle("Le serveur est en ligne!")
		},
	}} {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			if probe.check() == nil {
				t.Error("a defect this validator matches by value went unreported in " +
					"French, so the reach recorded for it is wrong")
			}
		})
	}
}

// The lapsing halves, pinned. Each row asserts that the English reply is caught
// and its translation is not, so the control proves the check works at all.
func TestTheWordAnchoredValidatorsStillLapseOnATranslation(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		name    string
		english string
		french  string
		check   func(string) error
	}{{
		name:    "a first-person action claim",
		english: "I filed a correction for review.",
		french:  "J'ai déposé une correction pour examen.",
		check: func(reply string) error {
			return ValidateGrounding(reply, "The current channel is #bots.")
		},
	}, {
		name:    "a social opening",
		english: "Hello there, the server is online.",
		french:  "Bonjour, le serveur est en ligne.",
		check:   ValidateNeutralStyle,
	}, {
		name:    "conversational personality framing",
		english: "Happy to help, let me know.",
		french:  "Ravi de vous aider, dites-moi.",
		check:   ValidateNeutralStyle,
	}, {
		name:    "a claim to be a person",
		english: "I am a real human being.",
		french:  "Je suis un véritable être humain.",
		check: func(reply string) error {
			return ValidateIdentityClaim(reply, PlaceholderPrincipal)
		},
	}, {
		name:    "the service naming its own tracker write",
		english: "Sirens Echo has filed a correction.",
		french:  "Sirens Echo a déposé une correction.",
		check: func(reply string) error {
			return ValidateSelfAttributedClaim(reply, "Sirens Echo", nil)
		},
	}} {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			if probe.check(probe.english) == nil {
				t.Fatalf("the English control passed, so this row proves nothing "+
					"about a translation: %q", probe.english)
			}
			if probe.check(probe.french) == nil {
				return
			}
			t.Errorf("%q is now rejected. If a language fix landed, move this row "+
				"to the surviving set and update validatorLanguageReach. See "+
				"sirens-echo#253", probe.french)
		})
	}
}

// A lapse recorded only in a test is a lapse a reader of the docs never meets,
// and the decision on sirens-echo#298 is priced by exactly this list.
func TestTheLanguageDocStatesWhichValidatorsLapse(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "sirens-echo-language.md"))
	if err != nil {
		t.Fatalf("read the language doc: %v", err)
	}
	flat := strings.Join(strings.Fields(string(body)), " ")
	for _, want := range []string{
		"ValidateGrounding", "ValidateNeutralStyle", "checkUserIDEcho", "issue 253",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the language doc does not name %s, so a reader pricing a "+
				"non-English channel cannot tell what it loses", want)
		}
	}
}
