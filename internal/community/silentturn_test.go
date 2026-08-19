package community

import (
	"context"
	"testing"
)

// answeredThroughToolClient is the shape a write tool aimed at the reply
// channel produces: the model answered through it and returned nothing.
type answeredThroughToolClient struct{}

func (answeredThroughToolClient) Complete(
	context.Context, TurnPrompt, string,
) (CompletionResult, error) {
	return CompletionResult{
		Content: "",
		ToolCalls: []ExecutedTool{{
			Name: "demo-discord__create_channel-message", Result: "sent", Outcome: ToolOutcomeOK,
		}},
	}, nil
}

func silentTurnAgent(client CompletionClient) *Agent {
	agent := &Agent{
		cfg:          Config{Definition: Definition{MaxContextMessages: 12}},
		completions:  client,
		systemPrompt: "neutral model policy and local knowledge",
		telemetry:    telemetryOrNoop(nil),
		slots:        make(chan struct{}, 1),
	}
	agent.ensureRuntimeDefaults()
	return agent
}

// The member saw two complete answers to one question, because the harness had
// no way for a turn to say it had already spoken. See sirens-echo#895.
func TestATurnThatAlreadyAnsweredThroughAToolPostsNothing(t *testing.T) {
	t.Parallel()
	agent := silentTurnAgent(answeredThroughToolClient{})
	turn := &httpTurn{requestID: "silent-turn", current: TranscriptEntry{
		Author: "member", Content: "what is the vibe check?",
	}}

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if turn.reply != "" {
		t.Errorf("the harness posted %q after the model had already spoken", turn.reply)
	}
}

// Silence stays distinguishable from a turn that simply produced nothing, which
// is the condition the decision reserved the right to reopen on.
func TestATurnThatDidNothingAtAllIsStillAFailure(t *testing.T) {
	t.Parallel()
	agent := silentTurnAgent(answeringClient{reply: ""})
	turn := &httpTurn{requestID: "empty-turn", current: TranscriptEntry{
		Author: "member", Content: "what is the vibe check?",
	}}

	err := agent.runTurn(context.Background(), turn, nil)
	if err == nil && turn.reply == "" {
		t.Error("a turn that ran no tool and said nothing passed as chosen silence")
	}
}
