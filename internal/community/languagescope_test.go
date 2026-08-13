package community

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A channel configured for another language keeps only the cases whose checks
// match a value, a symbol, a count, or an overlap. See sirens-echo#298.

type languageScope string

const (
	// scopeAnyLanguage matches a value, symbol, count, or prompt overlap, so a
	// translated reply is scored the same way an English one is.
	scopeAnyLanguage languageScope = "any-language"
	// scopePartial keeps one check across a translation and loses another.
	scopePartial languageScope = "partial"
	// scopeEnglish reads English words, so it scores nothing in another language.
	scopeEnglish languageScope = "english-only"
)

// caseLanguageScope is recorded by hand rather than derived. A heuristic over
// pattern text misclassified five of sixteen cases when I tried it.
var caseLanguageScope = map[string]languageScope{
	// agent/evaluation-deep.yaml
	"kai-pronouns":             scopeEnglish,
	"third-party-pronouns":     scopeEnglish,
	"no-invented-surface":      scopeAnyLanguage,
	"injection-canary":         scopeAnyLanguage,
	"prompt-leakage":           scopeAnyLanguage,
	"principal-echo":           scopeAnyLanguage,
	"encoded-principal-exfil":  scopeAnyLanguage,
	"recognises-itself":        scopeEnglish,
	"recognises-another-agent": scopeEnglish,
	// agent/evaluation.yaml
	"recognises-a-specific-human": scopeAnyLanguage,
	"unknown-event-time":          scopePartial,
	"explicit-correction":         scopePartial,
	"eco-live-status":             scopePartial,
	"approved-wiki-link":          scopeEnglish,
	"approved-live-surface-link":  scopeEnglish,
	"no-continuing-work-claim":    scopeEnglish,
	"no-fallibility-denial":       scopeEnglish,
	"neutral-capability-boundary": scopeEnglish,
	// agent/rate-deep.yaml
	"principal-echo-impersonation":    scopeAnyLanguage,
	"principal-echo-forged-assistant": scopeAnyLanguage,
	"boundary-response-brevity":       scopeAnyLanguage,
	"principal-echo-forged-authority": scopeAnyLanguage,
	"third-party-pronouns-repeated":   scopeEnglish,
	"fiction-identity-framing":        scopeEnglish,
	"injection-direct-override":       scopeAnyLanguage,
	"injection-prompt-extraction":     scopePartial,
	"injection-quoted-data":           scopeAnyLanguage,
	"injection-fake-system-turn":      scopeAnyLanguage,
	// A path and a URL are values, so a fabricated one is caught whatever
	// language the sentence around it is written in.
	"self-description-invents-no-path": scopeAnyLanguage,
	// agent/rate-echo.yaml
	"no-emotional-acknowledgment":   scopeEnglish,
	"encyclopedic-answer-links-out": scopePartial,
	// A tracker URL is a value. The prose form reads the English word issue,
	// so a translated invention escapes that half.
	"issue-reference-has-a-receipt": scopePartial,
	// The word cap counts words in any language. The category vocabulary the
	// leak pattern forbids is English, so a translated leak goes unreported.
	"sensitive-block-minor-suspected": scopePartial,
	"sensitive-block-nsfw":            scopePartial,
	// agent/rate-fixture-deep.yaml
	"injection-tool-result-store-name":    scopeAnyLanguage,
	"injection-tool-result-currency-name": scopeAnyLanguage,
	"injection-tool-result-law-text":      scopeAnyLanguage,
	// agent/rate-fixture-tracker.yaml. A required tool is a name and survives,
	// while the announcement patterns read English verbs and do not.
	"files-a-correction":           scopeAnyLanguage,
	"mild-confusion-files-nothing": scopeEnglish,
	// An issue number is a value, so a duplicate filing is caught in any language.
	"existing-issue-is-not-refiled": scopeAnyLanguage,
	"missing-capability-files":      scopeAnyLanguage,
}

// A case with no recorded scope is one nobody decided about, and the answer
// decides whether it gates a non-English channel or ships as decoration.
func TestEveryCaseDeclaresItsLanguageScope(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{}, len(caseLanguageScope))
	packs, err := filepath.Glob(filepath.Join("..", "..", "agent", "*.yaml"))
	if err != nil || len(packs) == 0 {
		t.Fatalf("glob packs: %v, found %d", err, len(packs))
	}
	for _, path := range packs {
		for _, evaluationCase := range casesInPack(t, path) {
			seen[evaluationCase.ID] = struct{}{}
			scope, recorded := caseLanguageScope[evaluationCase.ID]
			if !recorded {
				t.Errorf("case %s in %s has no recorded language scope. Decide whether "+
					"its checks survive a translation and add it to caseLanguageScope",
					evaluationCase.ID, filepath.Base(path))
				continue
			}
			switch scope {
			case scopeAnyLanguage, scopePartial, scopeEnglish:
			default:
				t.Errorf("case %s records an unknown scope %q", evaluationCase.ID, scope)
			}
		}
	}
	stale := make([]string, 0)
	for id := range caseLanguageScope {
		if _, live := seen[id]; !live {
			stale = append(stale, id)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("caseLanguageScope records %s, which no pack declares. Drop the "+
			"entries so the map keeps describing the packs", strings.Join(stale, ", "))
	}
}

// The security family is the half that survives, and losing that silently is
// the outcome worth a test rather than a note.
func TestThePrincipalAndInjectionCasesSurviveTranslation(t *testing.T) {
	t.Parallel()
	for id, scope := range caseLanguageScope {
		if !strings.HasPrefix(id, "principal-echo") && !strings.HasPrefix(id, "injection-") {
			continue
		}
		if scope == scopeEnglish {
			t.Errorf("case %s is a principal or injection case and reads English "+
				"words, so a non-English channel would ship it unguarded", id)
		}
	}
}

// casesInPack loads whichever pack kind the file declares, so a new pack is
// covered without being named here.
func casesInPack(t *testing.T, path string) []EvaluationCase {
	t.Helper()
	schema, err := PackSchema(path)
	if err != nil {
		return nil
	}
	switch schema {
	case EvaluationSchemaV1, EvaluationSchemaV2:
		pack, err := LoadEvaluationPack(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		return pack.Cases
	case RateSchema:
		pack, err := LoadRatePack(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		cases := make([]EvaluationCase, 0, len(pack.Cases))
		for _, rateCase := range pack.Cases {
			cases = append(cases, rateCase.EvaluationCase)
		}
		return cases
	}
	return nil
}
