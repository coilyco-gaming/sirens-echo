package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community/systemone"
)

func TestSnapReactionTakesOnlyAConfidentSocialMark(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		decision RouteDecision
		want     string
	}{
		{"social mark above cutoff", RouteDecision{Ran: true, Shape: "react:wave", ShapeProb: 0.9}, "wave"},
		{"below cutoff", RouteDecision{Ran: true, Shape: "react:wave", ShapeProb: 0.5}, ""},
		{"a fact needs a tool first", RouteDecision{Ran: true, Shape: "react:agree", ShapeProb: 0.99}, ""},
		{"full answer", RouteDecision{Ran: true, Shape: "full", ShapeProb: 0.99}, ""},
		{"a phrase is not a mark", RouteDecision{Ran: true, Shape: "phrase:not-permitted", ShapeProb: 0.99}, ""},
		{"stage never ran", RouteDecision{Shape: "react:wave", ShapeProb: 0.9}, ""},
		{"unknown key", RouteDecision{Ran: true, Shape: "react:shrug", ShapeProb: 0.9}, ""},
	}
	for _, c := range cases {
		key, ok := c.decision.SnapReaction()
		if key != c.want || ok != (c.want != "") {
			t.Errorf("%s: SnapReaction() = %q, %v, want %q", c.name, key, ok, c.want)
		}
	}
	fell := RouteDecision{Ran: true, Shape: "react:wave", ShapeProb: 0.9}
	fell.fellBackTo(RouteFamilyShape, jevFallbackMissingAnswer)
	if _, ok := fell.SnapReaction(); ok {
		t.Error("a fallen-back shape family snapped")
	}
}

func TestEverySnapKeyHasAGlyphAndAMeaning(t *testing.T) {
	t.Parallel()
	for key := range snapReactions {
		if replyReactions[key] == "" || reactionMeanings[key] == "" {
			t.Errorf("snap key %q lacks a glyph or a meaning", key)
		}
	}
	for key := range replyReactions {
		if reactionMeanings[key] == "" {
			t.Errorf("reaction %q has no meaning, so neither the model nor Jev can pick it for one", key)
		}
	}
}

// modelMustNotRun fails the test when the answer path reaches the model.
type modelMustNotRun struct{ t *testing.T }

func (c modelMustNotRun) Complete(context.Context, TurnPrompt, string) (CompletionResult, error) {
	c.t.Error("the model ran on a turn Jev snapped")
	return CompletionResult{Content: "words"}, nil
}

func snappingAgent(t *testing.T, shape string, probability float64) *Agent {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req systemone.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		resp := systemone.Response{}
		for _, q := range req.Questions {
			if q.Key == "shape" {
				resp.Answers = append(resp.Answers, systemone.Answer{Key: q.Key, Option: shape, Probability: probability})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)
	agent := silentTurnAgent(modelMustNotRun{t: t})
	agent.cfg.JevModel = "jev-latest"
	agent.cfg.AgentProxyURL = server.URL
	return agent
}

func TestAJevSnappedTurnMarksTheMessageWithNoModelCall(t *testing.T) {
	agent := snappingAgent(t, "react:heart", 0.95)
	turn := &markableTurn{}

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if !marked(turn, replyReactions["heart"]) {
		t.Errorf("marks = %v, want %q", turn.applied, replyReactions["heart"])
	}
	if len(turn.replies) != 0 {
		t.Errorf("replies = %v, want none beside the mark", turn.replies)
	}
}

// The turn tool cannot place a mark, so it returns the glyph and names the key.
func TestAJevSnapOverHTTPReturnsTheGlyphAndTheKey(t *testing.T) {
	agent := snappingAgent(t, "react:wave", 0.95)
	turn := &httpTurn{requestID: "snap", current: TranscriptEntry{Author: "member", Content: "hi"}}

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if turn.reply != replyReactions["wave"] || turn.reaction != "wave" {
		t.Errorf("reply, reaction = %q, %q, want %q, wave", turn.reply, turn.reaction, replyReactions["wave"])
	}
}

// A model-invoked mark reports its key the same way, so a caller reads one field.
func TestAModelInvokedMarkOverHTTPNamesTheKey(t *testing.T) {
	t.Parallel()
	agent := silentTurnAgent(answeringClient{reply: "{{react:agree}}"})
	turn := &httpTurn{requestID: "model-mark", current: TranscriptEntry{Author: "member", Content: "is the server up?"}}

	if err := agent.runTurn(context.Background(), turn, nil); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	if turn.reaction != "agree" || !strings.Contains(turn.reply, replyReactions["agree"]) {
		t.Errorf("reply, reaction = %q, %q, want the agree glyph and key", turn.reply, turn.reaction)
	}
}
