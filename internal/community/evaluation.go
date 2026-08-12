package community

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultEvaluationCaseTimeout = 5 * time.Minute

const (
	// EvaluationSchemaV1 is the original tool-and-phrase gate.
	EvaluationSchemaV1 = "sirens-discord-ops.evaluation.v1"
	// EvaluationSchemaV2 adds the scoped checks in evaluation_checks.go.
	EvaluationSchemaV2 = "sirens-discord-ops.evaluation.v2"
)

// EvaluationPack is the source-controlled live-path acceptance gate.
type EvaluationPack struct {
	Schema string           `json:"schema" yaml:"schema"`
	Cases  []EvaluationCase `json:"cases" yaml:"cases"`
}

// EvaluationCase exercises the same prompt and parser used by Discord without
// sending a Discord reply or creating a Forgejo issue.
type EvaluationCase struct {
	ID               string            `json:"id" yaml:"id"`
	History          []TranscriptEntry `json:"history" yaml:"history"`
	Current          TranscriptEntry   `json:"current" yaml:"current"`
	RequiredTool     string            `json:"required_tool" yaml:"required_tool"`
	ForbiddenPhrases []string          `json:"forbidden_phrases" yaml:"forbidden_phrases"`
	// Scoped and anchored checks. A whole-reply substring match cannot tell a
	// fabrication from a correct refusal quoting it, and these can.
	ForbiddenPatterns   []string      `json:"forbidden_patterns" yaml:"forbidden_patterns"`
	PronounPolicy       PronounPolicy `json:"pronoun_policy" yaml:"pronoun_policy"`
	MaxVerbatimWords    int           `json:"max_verbatim_words" yaml:"max_verbatim_words"`
	ForbidPrincipalEcho bool          `json:"forbid_principal_echo" yaml:"forbid_principal_echo"`

	compiledPatterns []*regexp.Regexp
}

// checked reports whether the case scores anything at all. A case with no check
// passes unconditionally, which reads as coverage it does not have.
func (c EvaluationCase) checked() bool {
	return c.RequiredTool != "" ||
		len(c.ForbiddenPhrases) > 0 ||
		len(c.ForbiddenPatterns) > 0 ||
		c.PronounPolicy.configured() ||
		c.MaxVerbatimWords > 0 ||
		c.ForbidPrincipalEcho
}

// PackSchema reads only the schema field so a caller can select the right
// loader. Loading the gate as the board silently is worse than a parse error.
func PackSchema(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read pack: %w", err)
	}
	var header struct {
		Schema string `yaml:"schema"`
	}
	if err := yaml.Unmarshal(raw, &header); err != nil {
		return "", fmt.Errorf("parse pack schema: %w", err)
	}
	return header.Schema, nil
}

// LoadEvaluationPack reads the deterministic deployment gate.
func LoadEvaluationPack(path string) (EvaluationPack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return EvaluationPack{}, fmt.Errorf("read evaluation pack: %w", err)
	}
	var pack EvaluationPack
	if err := yaml.Unmarshal(raw, &pack); err != nil {
		return EvaluationPack{}, fmt.Errorf("parse evaluation pack: %w", err)
	}
	// v1 and v2 differ only by the scoped checks, so a v1 pack keeps loading.
	if pack.Schema != EvaluationSchemaV1 && pack.Schema != EvaluationSchemaV2 {
		return EvaluationPack{}, fmt.Errorf("unsupported evaluation schema %q", pack.Schema)
	}
	if len(pack.Cases) == 0 {
		return EvaluationPack{}, fmt.Errorf("evaluation pack contains no cases")
	}
	for index := range pack.Cases {
		if err := prepareEvaluationCase(&pack.Cases[index]); err != nil {
			return EvaluationPack{}, err
		}
	}
	return pack, nil
}

// prepareEvaluationCase validates one case and compiles its patterns, so a bad
// expression fails the load rather than the deployment it was meant to guard.
func prepareEvaluationCase(evaluationCase *EvaluationCase) error {
	if evaluationCase.ID == "" || evaluationCase.Current.Content == "" {
		return fmt.Errorf("evaluation case requires id and current content")
	}
	if !evaluationCase.checked() {
		return fmt.Errorf("evaluation case %s scores nothing", evaluationCase.ID)
	}
	if err := evaluationCase.PronounPolicy.validate(evaluationCase.ID); err != nil {
		return err
	}
	if evaluationCase.MaxVerbatimWords < 0 {
		return fmt.Errorf("case %s max_verbatim_words cannot be negative", evaluationCase.ID)
	}
	compiled := make([]*regexp.Regexp, 0, len(evaluationCase.ForbiddenPatterns))
	for _, pattern := range evaluationCase.ForbiddenPatterns {
		expression, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("case %s pattern %q: %w", evaluationCase.ID, pattern, err)
		}
		compiled = append(compiled, expression)
	}
	evaluationCase.compiledPatterns = compiled
	return nil
}

// RunEvaluation executes the accepted Discord cases through Agent Proxy using
// the production prompt and structural validators. It performs no writes.
func RunEvaluation(
	ctx context.Context,
	definition Definition,
	principal Principal,
	localSkillpack string,
	pack EvaluationPack,
	completions CompletionClient,
	output io.Writer,
) error {
	return runEvaluation(
		ctx,
		definition,
		principal,
		localSkillpack,
		pack,
		completions,
		output,
		defaultEvaluationCaseTimeout,
	)
}

func runEvaluation(
	ctx context.Context,
	definition Definition,
	principal Principal,
	localSkillpack string,
	pack EvaluationPack,
	completions CompletionClient,
	output io.Writer,
	caseTimeout time.Duration,
) error {
	composed := ""
	if definition.Composed {
		composed = PlaceholderComposed
	}
	systemPrompt := BuildSystemPrompt(definition, principal, composed, localSkillpack)
	failures := make([]string, 0)
	for _, evaluationCase := range pack.Cases {
		prompt := BuildTurnPrompt(systemPrompt, evaluationCase.History, evaluationCase.Current)
		caseCtx, cancel := context.WithTimeout(ctx, caseTimeout)
		result, err := completions.Complete(caseCtx, prompt, evaluationCase.ID)
		cancel()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: inference: %v", evaluationCase.ID, err))
			continue
		}
		reply, err := ParseReply(result.Content)
		if err == nil {
			err = ValidateGrounding(reply, prompt.Supplied(), result.ToolCalls...)
		}
		if err == nil {
			err = ValidateIdentityClaim(reply, principal)
		}
		if err == nil {
			err = ValidateResponseStyle(definition.ResponseStyle, reply)
		}
		if err == nil && evaluationCase.RequiredTool != "" &&
			!completionUsedTool(result, evaluationCase.RequiredTool) {
			err = fmt.Errorf("expected tool %s", evaluationCase.RequiredTool)
		}
		if err == nil {
			lowerOutput := strings.ToLower(reply)
			for _, forbidden := range evaluationCase.ForbiddenPhrases {
				if strings.Contains(lowerOutput, strings.ToLower(forbidden)) {
					err = fmt.Errorf("contained forbidden phrase %q", forbidden)
					break
				}
			}
		}
		if err == nil {
			err = runScopedChecks(evaluationCase, reply, systemPrompt, principal)
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", evaluationCase.ID, err))
			continue
		}
		fmt.Fprintf(output, "%s: pass\n%s\n\n", evaluationCase.ID, reply)
	}
	if len(failures) > 0 {
		return fmt.Errorf("evaluation failed:\n%s", strings.Join(failures, "\n"))
	}
	return nil
}

// runScopedChecks applies the checks that need the reply's structure, the
// system prompt, or the principal rather than a bare substring.
func runScopedChecks(
	evaluationCase EvaluationCase,
	reply string,
	systemPrompt string,
	principal Principal,
) error {
	if err := checkForbiddenPatterns(reply, evaluationCase.compiledPatterns); err != nil {
		return err
	}
	if evaluationCase.PronounPolicy.configured() {
		if err := evaluationCase.PronounPolicy.check(reply); err != nil {
			return err
		}
	}
	if err := checkVerbatimLeak(reply, systemPrompt, evaluationCase.MaxVerbatimWords); err != nil {
		return err
	}
	if evaluationCase.ForbidPrincipalEcho {
		if err := checkPrincipalEcho(reply, principal); err != nil {
			return err
		}
	}
	return nil
}

func completionUsedTool(result CompletionResult, required string) bool {
	for _, call := range result.ToolCalls {
		if call.Name == required {
			return true
		}
	}
	return false
}
