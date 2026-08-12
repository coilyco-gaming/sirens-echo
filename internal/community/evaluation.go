package community

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultEvaluationCaseTimeout = 5 * time.Minute

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
	if pack.Schema != "sirens-discord-ops.evaluation.v1" {
		return EvaluationPack{}, fmt.Errorf("unsupported evaluation schema %q", pack.Schema)
	}
	if len(pack.Cases) == 0 {
		return EvaluationPack{}, fmt.Errorf("evaluation pack contains no cases")
	}
	for _, evaluationCase := range pack.Cases {
		if evaluationCase.ID == "" || evaluationCase.Current.Content == "" {
			return EvaluationPack{}, fmt.Errorf("evaluation case requires id and current content")
		}
		if evaluationCase.RequiredTool == "" &&
			len(evaluationCase.ForbiddenPhrases) == 0 {
			return EvaluationPack{}, fmt.Errorf(
				"evaluation case %s requires a tool or forbidden phrase",
				evaluationCase.ID,
			)
		}
	}
	return pack, nil
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

func completionUsedTool(result CompletionResult, required string) bool {
	for _, call := range result.ToolCalls {
		if call.Name == required {
			return true
		}
	}
	return false
}
