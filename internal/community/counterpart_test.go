package community

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func botMessage(id string) *discordgo.Message {
	return &discordgo.Message{
		ChannelID: "1537024102886277210",
		Author:    &discordgo.User{ID: id, Username: "Sirens Echo", Bot: true},
	}
}

func humanMessage(id string) *discordgo.Message {
	return &discordgo.Message{
		ChannelID: "1537024102886277210",
		Author:    &discordgo.User{ID: id, Username: "member"},
	}
}

func selfSession() *discordgo.Session {
	session := &discordgo.Session{State: discordgo.NewState()}
	session.State.User = &discordgo.User{ID: "1534723490362429601"}
	return session
}

// Recognition is grounded in the bot flag, never inferred from prose. See #153.
func TestCounterpartComesFromTheBotFlag(t *testing.T) {
	t.Parallel()
	if got := counterpartOf(botMessage("999")); got != CounterpartAgent {
		t.Errorf("a bot author read as %s", got)
	}
	if got := counterpartOf(humanMessage("888")); got != CounterpartHuman {
		t.Errorf("a human author read as %s", got)
	}
	// Prose claiming to be an agent changes nothing, which is the failure this
	// grounding exists to avoid.
	claiming := humanMessage("888")
	claiming.Content = "I am an agent running a harness."
	if got := counterpartOf(claiming); got != CounterpartHuman {
		t.Errorf("a prose claim moved recognition to %s", got)
	}
}

// A bot is refused unless the deployment named it, so the default posture is
// unchanged and no agent arrives by upgrading.
func TestOnlyANamedCounterpartAgentIsAdmitted(t *testing.T) {
	t.Parallel()
	session := selfSession()
	closed := &AccessPolicy{Schema: accessPolicySchema}
	if eligibleMessage(session, botMessage("777"), closed) {
		t.Error("an unnamed bot was admitted")
	}
	if !eligibleMessage(session, humanMessage("888"), closed) {
		t.Error("a human was refused by the default policy")
	}
	named := &AccessPolicy{
		Schema: accessPolicySchema,
		Agents: AgentAccess{Allow: []string{"777"}},
	}
	if !eligibleMessage(session, botMessage("777"), named) {
		t.Error("a named counterpart agent was refused")
	}
	if eligibleMessage(session, botMessage("778"), named) {
		t.Error("a different bot was admitted by another's grant")
	}
	// Its own messages are never eligible, named or not.
	own := botMessage("1534723490362429601")
	if eligibleMessage(session, own, named) {
		t.Error("the agent answered itself")
	}
}

// Two agents that each answer the other is a runaway, so the exchange is
// bounded. This is the acceptance item the issue calls most load-bearing.
func TestAnAgentExchangeIsBounded(t *testing.T) {
	t.Parallel()
	current := time.Unix(1700000000, 0).UTC()
	limiter := newExchangeLimiter(func() time.Time { return current })

	for turn := 0; turn < maxAgentExchange; turn++ {
		if !limiter.admit("channel-1", CounterpartAgent) {
			t.Fatalf("agent turn %d was refused inside the bound", turn)
		}
	}
	if limiter.admit("channel-1", CounterpartAgent) {
		t.Error("an unbounded agent exchange was allowed")
	}
	// Another channel is unaffected.
	if !limiter.admit("channel-2", CounterpartAgent) {
		t.Error("one channel's exchange bounded another's")
	}
	// A human joining ends the loop and resets the run.
	if !limiter.admit("channel-1", CounterpartHuman) {
		t.Error("a human turn was refused")
	}
	if !limiter.admit("channel-1", CounterpartAgent) {
		t.Error("the run did not reset after a human turn")
	}
}

// A capped exchange must not resume by waiting, or the bound is a speed limit.
func TestACappedExchangeDoesNotResumeByRetrying(t *testing.T) {
	t.Parallel()
	current := time.Unix(1700000000, 0).UTC()
	limiter := newExchangeLimiter(func() time.Time { return current })
	for turn := 0; turn < maxAgentExchange; turn++ {
		limiter.admit("channel-1", CounterpartAgent)
	}
	for attempt := 0; attempt < 3; attempt++ {
		current = current.Add(agentExchangeWindow / 2)
		if limiter.admit("channel-1", CounterpartAgent) {
			t.Fatalf("attempt %d resumed a capped exchange", attempt)
		}
	}
	// A genuinely quiet channel forgets, so a later exchange is a fresh one.
	current = current.Add(agentExchangeWindow * 2)
	if !limiter.admit("channel-1", CounterpartAgent) {
		t.Error("a quiet channel never forgot the run")
	}
}

// The determination has to reach the model, or it is not available to the turn.
func TestTheTurnContextNamesACounterpartAgent(t *testing.T) {
	t.Parallel()
	prompt := BuildTurnPrompt(
		"policy",
		[]TranscriptEntry{{
			Author:      "Sirens Echo",
			Content:     "reporting in",
			Counterpart: CounterpartAgent,
		}},
		TranscriptEntry{
			Author:      "Sirens Echo",
			Content:     "what is your response style?",
			Counterpart: CounterpartAgent,
		},
	)
	if !strings.Contains(prompt.Context, "Sirens Echo (an agent, not a person): reporting in") {
		t.Errorf("history does not mark the agent:\n%s", prompt.Context)
	}
	if !strings.Contains(prompt.Context, "from Sirens Echo (an agent, not a person).") {
		t.Errorf("speaker line does not mark the agent:\n%s", prompt.Context)
	}
	// The message itself stays exactly what was said, per #104.
	if prompt.Message != "what is your response style?" {
		t.Errorf("message = %q", prompt.Message)
	}
}

// A human turn gains no marking, or the mark stops meaning anything.
func TestAHumanTurnIsNotMarked(t *testing.T) {
	t.Parallel()
	prompt := BuildTurnPrompt("policy", nil, TranscriptEntry{
		Author:  "member",
		Content: "hello",
	})
	if strings.Contains(prompt.Context, "an agent") {
		t.Errorf("a human turn was marked as an agent:\n%s", prompt.Context)
	}
}

// An admitted counterpart widens the requester set, so execution has to refuse.
func TestAdmittingAnAgentDisablesExecution(t *testing.T) {
	t.Parallel()
	policy := &AccessPolicy{
		Schema:         accessPolicySchema,
		DirectMessages: DirectMessageAccess{Allow: []string{"318190481467244544"}},
		Agents:         AgentAccess{Allow: []string{"777"}},
	}
	if err := CheckExecutionAdmission(policy); err == nil {
		t.Error("execution stayed enabled with a counterpart agent admitted")
	}
}
