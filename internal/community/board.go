package community

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// BoardSchema names the human-graded pack. The deterministic gate keeps
// sirens-discord-ops.evaluation.v1 and the two never share a file.
const BoardSchema = "sirens-discord-ops.board.v1"

// BoardDatasetSchema names the emitted annotation input.
const BoardDatasetSchema = "sirens-discord-ops.board-dataset.v1"

const (
	// BoardHalfIn is the half where the clause requires the agent to act.
	BoardHalfIn = "in"
	// BoardHalfOut is the half where the clause requires the agent to decline.
	BoardHalfOut = "out"
)

// BoardCase is one authored case. Target states what passing means in prose,
// because a human grades this pack and a phrase list is not a criterion.
type BoardCase struct {
	ID           string            `yaml:"id"`
	Clause       string            `yaml:"clause"`
	Half         string            `yaml:"half"`
	PairID       string            `yaml:"pair_id"`
	History      []TranscriptEntry `yaml:"history"`
	Current      TranscriptEntry   `yaml:"current"`
	Target       string            `yaml:"target"`
	RequiredTool string            `yaml:"required_tool"`
}

// BoardPack is the source-controlled human-graded board. It never gates a
// deployment, so nothing in this file decides pass or fail.
type BoardPack struct {
	Schema string      `yaml:"schema"`
	Cases  []BoardCase `yaml:"cases"`
}

// BoardProvenance records what produced a dataset. Evaluation evidence without
// its exact substrate is not reproducible, so the runner refuses to omit it.
type BoardProvenance struct {
	Definition  string `yaml:"definition"`
	Pack        string `yaml:"pack"`
	Model       string `yaml:"model"`
	Transport   string `yaml:"transport"`
	Roster      string `yaml:"roster"`
	Epochs      int    `yaml:"epochs"`
	Composed    string `yaml:"composed"`
	GeneratedAt string `yaml:"generated_at"`
}

// BoardResponse is one subject run. Structural carries the deployed validators'
// verdict as evidence rather than as a score. See docs/sirens-echo-board.md.
type BoardResponse struct {
	Epoch      int      `yaml:"epoch"`
	Text       string   `yaml:"text"`
	Tools      []string `yaml:"tools,omitempty"`
	Structural string   `yaml:"structural,omitempty"`
	Error      string   `yaml:"error,omitempty"`
}

// BoardRecord is one case carrying the responses a grader annotates.
type BoardRecord struct {
	ID           string          `yaml:"id"`
	Clause       string          `yaml:"clause"`
	Half         string          `yaml:"half"`
	PairID       string          `yaml:"pair_id"`
	Target       string          `yaml:"target"`
	RequiredTool string          `yaml:"required_tool,omitempty"`
	Responses    []BoardResponse `yaml:"responses"`
}

// BoardDataset is the annotation input. Go emits it and a grader consumes it.
type BoardDataset struct {
	Schema     string          `yaml:"schema"`
	Provenance BoardProvenance `yaml:"provenance"`
	Records    []BoardRecord   `yaml:"records"`
}

// LoadBoardPack reads and validates the human-graded board.
func LoadBoardPack(path string) (BoardPack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return BoardPack{}, fmt.Errorf("read board pack: %w", err)
	}
	var pack BoardPack
	if err := yaml.Unmarshal(raw, &pack); err != nil {
		return BoardPack{}, fmt.Errorf("parse board pack: %w", err)
	}
	if pack.Schema != BoardSchema {
		return BoardPack{}, fmt.Errorf("unsupported board schema %q", pack.Schema)
	}
	if len(pack.Cases) == 0 {
		return BoardPack{}, fmt.Errorf("board pack contains no cases")
	}
	if err := validateBoardCases(pack.Cases); err != nil {
		return BoardPack{}, err
	}
	return pack, nil
}

// validateBoardCases enforces the pair invariant at load time. A pair holding
// one half is the shape that silently deletes a finding, so it fails here.
func validateBoardCases(cases []BoardCase) error {
	seen := make(map[string]struct{}, len(cases))
	halves := make(map[string]map[string]string)
	for _, boardCase := range cases {
		switch {
		case boardCase.ID == "":
			return fmt.Errorf("board case requires an id")
		case boardCase.Clause == "":
			return fmt.Errorf("board case %s requires a clause", boardCase.ID)
		case strings.TrimSpace(boardCase.Current.Content) == "":
			return fmt.Errorf("board case %s requires current content", boardCase.ID)
		case strings.TrimSpace(boardCase.Target) == "":
			return fmt.Errorf("board case %s requires a target", boardCase.ID)
		case boardCase.Half != BoardHalfIn && boardCase.Half != BoardHalfOut:
			return fmt.Errorf(
				"board case %s half must be %q or %q",
				boardCase.ID, BoardHalfIn, BoardHalfOut,
			)
		case boardCase.PairID == "":
			return fmt.Errorf("board case %s requires a pair_id", boardCase.ID)
		}
		if _, duplicate := seen[boardCase.ID]; duplicate {
			return fmt.Errorf("board case %s is declared twice", boardCase.ID)
		}
		seen[boardCase.ID] = struct{}{}
		pair, ok := halves[boardCase.PairID]
		if !ok {
			pair = make(map[string]string, 2)
			halves[boardCase.PairID] = pair
		}
		if existing, taken := pair[boardCase.Half]; taken {
			return fmt.Errorf(
				"pair %s declares half %s twice, in %s and %s",
				boardCase.PairID, boardCase.Half, existing, boardCase.ID,
			)
		}
		pair[boardCase.Half] = boardCase.ID
	}
	return validateBoardPairsComplete(halves)
}

func validateBoardPairsComplete(halves map[string]map[string]string) error {
	incomplete := make([]string, 0)
	for pairID, pair := range halves {
		if len(pair) != 2 {
			incomplete = append(incomplete, pairID)
		}
	}
	if len(incomplete) > 0 {
		sort.Strings(incomplete)
		return fmt.Errorf(
			"pairs missing a half: %s. The in half is the negative control and is never optional",
			strings.Join(incomplete, ", "),
		)
	}
	return nil
}

// RunBoard executes every case for the requested epochs and writes the dataset
// a human annotates. It reports no verdict. See docs/sirens-echo-board.md.
func RunBoard(
	ctx context.Context,
	definition Definition,
	principal Principal,
	localSkillpack string,
	pack BoardPack,
	provenance BoardProvenance,
	completions CompletionClient,
	output io.Writer,
) error {
	return runBoard(
		ctx,
		definition,
		principal,
		localSkillpack,
		pack,
		provenance,
		completions,
		output,
		defaultEvaluationCaseTimeout,
	)
}

func runBoard(
	ctx context.Context,
	definition Definition,
	principal Principal,
	localSkillpack string,
	pack BoardPack,
	provenance BoardProvenance,
	completions CompletionClient,
	output io.Writer,
	caseTimeout time.Duration,
) error {
	epochs := provenance.Epochs
	if epochs < 1 {
		return fmt.Errorf("board run requires at least one epoch")
	}
	composed, composedState, err := composedForRun(definition)
	if err != nil {
		return err
	}
	systemPrompt, err := evaluationSystemPrompt(definition, principal, composed, localSkillpack)
	if err != nil {
		return err
	}
	// Derived rather than accepted, so a graded dataset cannot name a bundle
	// this run did not read. See sirens-echo#316.
	provenance.Composed = composedState
	dataset := BoardDataset{
		Schema:     BoardDatasetSchema,
		Provenance: provenance,
		Records:    make([]BoardRecord, 0, len(pack.Cases)),
	}
	silent := make([]string, 0)
	for _, boardCase := range pack.Cases {
		prompt := BuildTurnPrompt(systemPrompt, boardCase.History, boardCase.Current)
		record := BoardRecord{
			ID:           boardCase.ID,
			Clause:       boardCase.Clause,
			Half:         boardCase.Half,
			PairID:       boardCase.PairID,
			Target:       boardCase.Target,
			RequiredTool: boardCase.RequiredTool,
			Responses:    make([]BoardResponse, 0, epochs),
		}
		answered := false
		for epoch := 1; epoch <= epochs; epoch++ {
			response := runBoardEpoch(
				ctx,
				boardCase,
				epoch,
				definition.ResponseStyle,
				prompt,
				completions,
				caseTimeout,
			)
			if response.Error == "" {
				answered = true
			}
			record.Responses = append(record.Responses, response)
		}
		if !answered {
			silent = append(silent, boardCase.ID)
		}
		dataset.Records = append(dataset.Records, record)
	}
	encoded, err := yaml.Marshal(dataset)
	if err != nil {
		return fmt.Errorf("encode board dataset: %w", err)
	}
	if _, err := output.Write(encoded); err != nil {
		return fmt.Errorf("write board dataset: %w", err)
	}
	if len(silent) > 0 {
		return fmt.Errorf(
			"no response in any epoch for: %s",
			strings.Join(silent, ", "),
		)
	}
	return nil
}

func runBoardEpoch(
	ctx context.Context,
	boardCase BoardCase,
	epoch int,
	responseStyle string,
	prompt TurnPrompt,
	completions CompletionClient,
	caseTimeout time.Duration,
) BoardResponse {
	response := BoardResponse{Epoch: epoch}
	caseCtx, cancel := context.WithTimeout(ctx, caseTimeout)
	// The request id separates epochs so a transport trace resolves to one run.
	result, err := completions.Complete(
		caseCtx,
		prompt,
		fmt.Sprintf("%s#%d", boardCase.ID, epoch),
	)
	cancel()
	if err != nil {
		response.Error = fmt.Sprintf("inference: %v", err)
		return response
	}
	for _, call := range result.ToolCalls {
		response.Tools = append(response.Tools, call.Name)
	}
	reply, err := ParseReply(result.Content)
	if err != nil {
		// Preserve the raw content. A reply the parser rejects is still the
		// behavior under study and deleting it would drop the evidence.
		response.Text = result.Content
		response.Structural = err.Error()
		return response
	}
	response.Text = reply
	response.Structural = boardStructuralNote(
		boardCase,
		responseStyle,
		reply,
		prompt.Supplied(),
		result,
	)
	return response
}

// boardStructuralNote records what the deployed validators say. It never
// decides the case, for the measured reason in docs/sirens-echo-board.md.
func boardStructuralNote(
	boardCase BoardCase,
	responseStyle string,
	reply string,
	supplied string,
	result CompletionResult,
) string {
	notes := make([]string, 0, 3)
	if err := ValidateGrounding(reply, supplied, result.ToolCalls...); err != nil {
		notes = append(notes, err.Error())
	}
	if err := ValidateResponseStyle(responseStyle, reply); err != nil {
		notes = append(notes, err.Error())
	}
	if boardCase.RequiredTool != "" && !completionUsedTool(result, boardCase.RequiredTool) {
		notes = append(notes, fmt.Sprintf("expected tool %s", boardCase.RequiredTool))
	}
	return strings.Join(notes, " | ")
}
