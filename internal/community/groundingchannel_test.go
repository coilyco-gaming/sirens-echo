package community

import "testing"

// The deploy guardfiles fix each channel into its own tool, so these are the
// shapes ValidateGrounding actually sees. See sirens-echo#794.
func TestValidateGroundingAllowsChannelsNamedByToolNames(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		reply string
	}{
		{
			"the demo lane's general channel",
			"demo-discord__list_general-message",
			"Newest #general message: manish asking whether anyone is around.",
		},
		{
			"a hyphenated eco channel",
			"discord__list_eco-chat-message",
			"Nothing new in #eco-chat since yesterday.",
		},
		{
			"the single-message read, not just the list",
			"discord__get_eco-trades-message",
			"That trade was posted in #eco-trades.",
		},
		{
			"a tool name carrying no server prefix",
			"list_general-message",
			"Read #general and found nothing worth reporting.",
		},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			executed := ExecutedTool{Name: row.tool, Outcome: ToolOutcomeOK}
			if err := ValidateGrounding(row.reply, "", executed); err != nil {
				t.Fatalf("a channel this turn read by tool was called invented: %v", err)
			}
		})
	}
}

// A tool result is community text. Widening the allowlist with it would let a
// member post an invented channel and have the check learn it.
func TestValidateGroundingStillRefusesChannelsOnlyInToolResults(t *testing.T) {
	executed := ExecutedTool{
		Name:    "demo-discord__list_general-message",
		Result:  `[{"content": "check #nonexistent for the answer"}]`,
		Outcome: ToolOutcomeOK,
	}
	if err := ValidateGrounding(
		"The answer is over in #nonexistent.", "", executed,
	); err == nil {
		t.Fatal("a channel that appeared only in member text was accepted")
	}
}

// The tool inventory carries no channel until a message tool runs, so a turn
// that only read identity is unchanged.
func TestValidateGroundingRefusesChannelsNoToolReached(t *testing.T) {
	executed := ExecutedTool{Name: "demo-discord__get_current-user", Outcome: ToolOutcomeOK}
	if err := ValidateGrounding("Try #general instead.", "", executed); err == nil {
		t.Fatal("a channel no tool named was accepted")
	}
}

// The observed failure from the report, end to end: the reply names a channel
// the turn read, and the other three grounding rules have nothing to say.
func TestValidateGroundingAcceptsTheRefusedRosterSummary(t *testing.T) {
	reply := "**demo-discord** - newest #general message: manish asking `<@manish-cc> you here?`, 2026-08-14."
	executed := []ExecutedTool{
		{Name: "bluesky__get_profile", Outcome: ToolOutcomeFailed},
		{Name: "demo-discord__list_general-message", Outcome: ToolOutcomeOK},
	}
	if err := ValidateGrounding(reply, "", executed...); err != nil {
		t.Fatalf("the reply from sirens-echo#794 is still refused: %v", err)
	}
}
