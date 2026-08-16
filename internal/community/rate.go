package community

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RateSchema names the non-gating pack that measures an intermittent behavior.
const RateSchema = "sirens-discord-ops.rate.v1"

// RateDatasetSchema names the emitted evidence.
const RateDatasetSchema = "sirens-discord-ops.rate-dataset.v1"

// Run outcomes. An error is a transient of the substrate rather than a
// behavior, so it is reported and left out of the denominator.
const (
	RateOutcomePass  = "pass"
	RateOutcomeFail  = "fail"
	RateOutcomeError = "error"
)

// RateCase is a gate case plus the two numbers a rate needs. It embeds
// EvaluationCase so a case moves between the packs without being rewritten.
type RateCase struct {
	EvaluationCase `yaml:",inline"`
	// Runs is this case's repetition count. A 13 percent behavior and a 40
	// percent behavior need different N to separate from noise.
	Runs int `yaml:"runs"`
	// MaxFailureRate is the rate this case must beat, as a fraction. Zero
	// means every attempt must pass.
	MaxFailureRate float64 `yaml:"max_failure_rate"`
	// Observed records the rate that motivated the case, for the reader.
	Observed string `yaml:"observed,omitempty"`
	// Shape classifies the case for the relative brevity comparison, by what a
	// correct reply looks like. See docs/sirens-echo-brevity.md.
	Shape string `yaml:"shape,omitempty"`
}

// RatePack is the source-controlled measurement pack. It gates no deployment.
type RatePack struct {
	Schema string     `yaml:"schema"`
	Cases  []RateCase `yaml:"cases"`
}

// RateProvenance records the substrate. A rate without it is not reproducible
// and not comparable to the next run, so the runner refuses to omit it.
type RateProvenance struct {
	Definition string `yaml:"definition"`
	Pack       string `yaml:"pack"`
	Model      string `yaml:"model"`
	Transport  string `yaml:"transport"`
	Roster     string `yaml:"roster"`
	// Fixture names the tool fixture, exclusive with the roster. Without it a
	// fixture run reads as roster: empty. See docs/sirens-echo-rate-provenance.md.
	Fixture string `yaml:"fixture"`
	// Substrate records host state at run time. A contended GPU returns a
	// complete but degraded reply, which no in-process check can detect.
	Substrate string `yaml:"substrate"`
	// Runner is the commit behind the prompt, definition, skillpack and checks.
	// A pair whose halves cannot be attributed to commits is not a comparison.
	Runner string `yaml:"runner"`
	// Composed says whether the agent-compose bundle was real or a stub, which
	// bounds the rate. See docs/sirens-echo-rate-provenance.md.
	Composed string `yaml:"composed"`
	// Image records a deployed image only when one participates in the run. This
	// runner never calls a pod, so it stays unrecorded. Read Runner instead.
	Image       string `yaml:"image"`
	GeneratedAt string `yaml:"generated_at"`
}

// SubstrateUnrecorded marks a dataset nobody described the host for. Saying so
// beats letting a later reader assume it was idle. See docs/sirens-echo-sweep.md.
const SubstrateUnrecorded = "unrecorded"

// ImageUnrecorded marks a dataset whose build is unknown, which makes it
// unusable for before-and-after comparison. See docs/sirens-echo-sweep.md.
const ImageUnrecorded = "unrecorded"

// FixtureNone marks a run that served no fixture, so the absence is a statement
// rather than a missing field. See sirens-echo#311.
const FixtureNone = "none"

// RunnerUnrecorded marks a dataset built without a revision stamp. A rate that
// cannot name the checkout that produced it cannot be compared to another.
const RunnerUnrecorded = "unrecorded"

// Composed states for a dataset. A stubbed run measures this configuration and
// not the deployed service, which is a bound on the rate rather than a defect.
const (
	ComposedStubbed      = "stubbed placeholder"
	ComposedNotRequested = "not composed"
)

// ComposedBundleEnv opts a run into a real bundle. Unset keeps the stub, so an
// existing run does not change meaning. See docs/sirens-echo-compose.md.
const ComposedBundleEnv = "SIRENS_ECHO_COMPOSED_BUNDLE"

// RateRun is one attempt. Text is kept because a failure is not confirmed
// until a human reads the reply, and a checker defect looks like a finding.
type RateRun struct {
	Run int `yaml:"run"`
	// Model is what actually served this attempt. A fallback answers as a
	// different model, and a rate attributed to the wrong one is not a rate.
	Model   string `yaml:"model,omitempty"`
	Outcome string `yaml:"outcome"`
	// Detail is the first failure, which is what the gate would have reported.
	Detail string `yaml:"detail,omitempty"`
	// Details is every failure. A rate built from Detail alone attributes an
	// attempt to one check and hides the rest. See issue 304.
	Details []string `yaml:"details,omitempty"`
	Text    string   `yaml:"text,omitempty"`
	Tools   []string `yaml:"tools,omitempty"`
}

// RateRecord is one case's measurement.
type RateRecord struct {
	ID             string    `yaml:"id"`
	Runs           int       `yaml:"runs"`
	Attempts       int       `yaml:"attempts"`
	Passed         int       `yaml:"passed"`
	Failed         int       `yaml:"failed"`
	Errors         int       `yaml:"errors"`
	FailureRate    float64   `yaml:"failure_rate"`
	MaxFailureRate float64   `yaml:"max_failure_rate"`
	Observed       string    `yaml:"observed,omitempty"`
	Breached       bool      `yaml:"breached"`
	Measured       bool      `yaml:"measured"`
	Responses      []RateRun `yaml:"responses"`
}

// RateDataset is the emitted evidence. It carries every reply, so a reader can
// confirm a failure was real rather than a defect in the check that found it.
type RateDataset struct {
	Schema     string         `yaml:"schema"`
	Provenance RateProvenance `yaml:"provenance"`
	Records    []RateRecord   `yaml:"records"`
	// Brevity is the cross-case comparison, which no per-reply check can make.
	Brevity RateBrevity `yaml:"brevity"`
}

// LoadRatePack reads and validates the measurement pack.
func LoadRatePack(path string) (RatePack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RatePack{}, fmt.Errorf("read rate pack: %w", err)
	}
	var pack RatePack
	if err := yaml.Unmarshal(raw, &pack); err != nil {
		return RatePack{}, fmt.Errorf("parse rate pack: %w", err)
	}
	if pack.Schema != RateSchema {
		return RatePack{}, fmt.Errorf("unsupported rate schema %q", pack.Schema)
	}
	if len(pack.Cases) == 0 {
		return RatePack{}, fmt.Errorf("rate pack contains no cases")
	}
	for index := range pack.Cases {
		if err := prepareRateCase(&pack.Cases[index]); err != nil {
			return RatePack{}, err
		}
	}
	return pack, nil
}

// prepareRateCase reuses the gate's own validation so a case cannot mean one
// thing here and another there.
func prepareRateCase(rateCase *RateCase) error {
	if err := prepareEvaluationCase(&rateCase.EvaluationCase); err != nil {
		return err
	}
	if rateCase.Runs < 1 {
		return fmt.Errorf("rate case %s requires runs of at least 1", rateCase.ID)
	}
	if rateCase.MaxFailureRate < 0 || rateCase.MaxFailureRate > 1 {
		return fmt.Errorf(
			"rate case %s max_failure_rate must be a fraction between 0 and 1, got %v",
			rateCase.ID, rateCase.MaxFailureRate,
		)
	}
	if !validShape(rateCase.Shape) {
		return fmt.Errorf(
			"rate case %s shape must be %q, %q, or unset, got %q",
			rateCase.ID, ShapeBoundary, ShapeConversational, rateCase.Shape,
		)
	}
	return nil
}

// RunRate measures each case over its own repetition count and writes the
// dataset. It fails only on a declared threshold. See docs/sirens-echo-rate.md.
func RunRate(
	ctx context.Context,
	definition Definition,
	principal Principal,
	localSkillpack string,
	pack RatePack,
	provenance RateProvenance,
	completions CompletionClient,
	output io.Writer,
) error {
	return runRate(
		ctx, definition, principal, localSkillpack,
		pack, provenance, completions, output, defaultEvaluationCaseTimeout,
	)
}

func runRate(
	ctx context.Context,
	definition Definition,
	principal Principal,
	localSkillpack string,
	pack RatePack,
	provenance RateProvenance,
	completions CompletionClient,
	output io.Writer,
	caseTimeout time.Duration,
) error {
	// Derived where the substitution happens, so a caller cannot claim a bundle
	// the run did not use.
	composed, composedState, err := composedForRun(definition)
	if err != nil {
		return err
	}
	provenance.Composed = composedState
	systemPrompt, err := evaluationSystemPrompt(definition, principal, composed, localSkillpack)
	if err != nil {
		return err
	}
	if strings.TrimSpace(provenance.Substrate) == "" {
		provenance.Substrate = SubstrateUnrecorded
	}
	if strings.TrimSpace(provenance.Image) == "" {
		provenance.Image = ImageUnrecorded
	}
	dataset := RateDataset{
		Schema:     RateDatasetSchema,
		Provenance: provenance,
		Records:    make([]RateRecord, 0, len(pack.Cases)),
	}
	for _, rateCase := range pack.Cases {
		// A cut run still emits what it measured. Timeouts against a wedged route
		// are results, and discarding them costs the next run the same wait.
		if ctx.Err() != nil {
			break
		}
		record := measureRateCase(
			ctx, rateCase, definition, principal,
			systemPrompt, completions, caseTimeout,
		)
		if record.Errors > 0 {
			// A clean verdict can rest on fewer runs than declared, so say so in
			// the run log's own format. See docs/sirens-echo-rate-errors.md.
			slog.New(slog.NewJSONHandler(os.Stderr, nil)).Warn("rate.sample.decimated",
				"case", record.ID,
				"declared_runs", record.Runs,
				"scored_attempts", record.Attempts,
				"errors", record.Errors,
			)
		}
		dataset.Records = append(dataset.Records, record)
	}
	cutShort := ctx.Err()
	dataset.Brevity = measureRateBrevity(pack, dataset.Records)
	encoded, err := yaml.Marshal(dataset)
	if err != nil {
		return fmt.Errorf("encode rate dataset: %w", err)
	}
	if _, err := output.Write(encoded); err != nil {
		return fmt.Errorf("write rate dataset: %w", err)
	}
	if cutShort != nil {
		return fmt.Errorf(
			"rate run cut short after %d of %d cases, dataset written: %w",
			len(dataset.Records), len(pack.Cases), cutShort,
		)
	}
	return rateVerdict(dataset.Records, dataset.Brevity)
}

func measureRateCase(
	ctx context.Context,
	rateCase RateCase,
	definition Definition,
	principal Principal,
	systemPrompt string,
	completions CompletionClient,
	caseTimeout time.Duration,
) RateRecord {
	prompt := BuildTurnPrompt(systemPrompt, rateCase.promptHistory(), rateCase.Current)
	record := RateRecord{
		ID:             rateCase.ID,
		Runs:           rateCase.Runs,
		MaxFailureRate: rateCase.MaxFailureRate,
		Observed:       rateCase.Observed,
		Responses:      make([]RateRun, 0, rateCase.Runs),
	}
	for run := 1; run <= rateCase.Runs; run++ {
		outcome := runRateAttempt(
			ctx, rateCase, run, definition.ResponseStyle, definition.Identity,
			definition.SelfAliases, prompt, systemPrompt, principal, completions,
			caseTimeout,
		)
		switch outcome.Outcome {
		case RateOutcomePass:
			record.Passed++
		case RateOutcomeFail:
			record.Failed++
		default:
			record.Errors++
		}
		record.Responses = append(record.Responses, outcome)
	}
	record.Attempts = record.Passed + record.Failed
	record.Measured = record.Attempts > 0
	if record.Measured {
		record.FailureRate = float64(record.Failed) / float64(record.Attempts)
		record.Breached = record.FailureRate > record.MaxFailureRate
	}
	return record
}

func runRateAttempt(
	ctx context.Context,
	rateCase RateCase,
	run int,
	responseStyle string,
	identity string,
	aliases []string,
	prompt TurnPrompt,
	systemPrompt string,
	principal Principal,
	completions CompletionClient,
	caseTimeout time.Duration,
) RateRun {
	attemptCtx, cancel := context.WithTimeout(ctx, caseTimeout)
	defer cancel()
	// A distinct request id per run, so one attempt is traceable on its own.
	result, err := completions.Complete(
		attemptCtx, prompt, fmt.Sprintf("%s#%d", rateCase.ID, run),
	)
	if err != nil {
		// The substrate failed rather than the agent. Reported, not counted.
		return RateRun{Run: run, Outcome: RateOutcomeError, Detail: err.Error()}
	}
	// Every failing check, not the first. A rate attributed to one reason hides
	// the others, and severity is not the order they run in. See issue 304.
	reply, scoreErrs := ScoreEvaluationCaseAll(
		rateCase.EvaluationCase, result, prompt, systemPrompt, responseStyle, identity,
		aliases, principal,
	)
	attempt := RateRun{
		Run:   run,
		Model: result.ServedModel,
		Text:  reply,
		Tools: toolNames(result),
	}
	if len(scoreErrs) > 0 {
		attempt.Outcome = RateOutcomeFail
		attempt.Detail = scoreErrs[0].Error()
		attempt.Details = failureDetails(scoreErrs)
		return attempt
	}
	attempt.Outcome = RateOutcomePass
	return attempt
}

// failureDetails renders every failing check, in the order the gate runs them,
// so a reader can see what a first-failure attribution left out.
func failureDetails(failures []error) []string {
	details := make([]string, 0, len(failures))
	for _, failure := range failures {
		details = append(details, failure.Error())
	}
	return details
}

func toolNames(result CompletionResult) []string {
	if len(result.ToolCalls) == 0 {
		return nil
	}
	names := make([]string, 0, len(result.ToolCalls))
	for _, call := range result.ToolCalls {
		names = append(names, call.Name)
	}
	return names
}

// rateVerdict fails on a breached threshold and on a case with no attempts.
// An unmeasured case is not a pass. See docs/sirens-echo-rate.md.
func rateVerdict(records []RateRecord, brevity RateBrevity) error {
	problems := make([]string, 0)
	for _, record := range records {
		if !record.Measured {
			problems = append(problems, fmt.Sprintf(
				"%s: not measured, all %d runs errored", record.ID, record.Errors,
			))
			continue
		}
		if record.Breached {
			problems = append(problems, fmt.Sprintf(
				"%s: %d/%d failed (%.1f%%) against a %.1f%% ceiling, "+
					"%d of %d declared runs errored and are excluded",
				record.ID, record.Failed, record.Attempts,
				record.FailureRate*100, record.MaxFailureRate*100,
				record.Errors, record.Runs,
			))
		}
	}
	if problem := brevityProblem(brevity); problem != "" {
		problems = append(problems, problem)
	}
	if len(problems) > 0 {
		return fmt.Errorf("rate run:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}
