package community

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A prompt name is server-supplied and satisfies none of Discord's shape by
// construction. A malformed set costs every command in it. See issue 127.

func TestAServerPromptNameBecomesAUsableCommandName(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct{ prompt, want string }{
		{"Summarize Channel", "summarize-channel"},
		{"eco.trade_lookup", "eco-trade_lookup"},
		{"UPPER", "upper"},
		{"  padded  ", "padded"},
	} {
		if got := commandNameFor(probe.prompt); got != probe.want {
			t.Errorf("commandNameFor(%q) = %q, want %q", probe.prompt, got, probe.want)
		}
	}
}

// Refusing beats repairing. A name that cleans to nothing has no honest
// command form, and inventing one would register a command nobody declared.
func TestAPromptWithNoUsableNameIsRefused(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "***", "   ", "日本語"} {
		if _, err := CommandFromPrompt(&mcp.Prompt{
			Name: name, Description: "a description",
		}); err == nil {
			t.Errorf("prompt name %q produced a command", name)
		}
	}
}

// Discord refuses a description over 100 runes, and refusing the whole
// registration for one long prompt would cost every other command.
func TestALongDescriptionIsTruncatedNotRefused(t *testing.T) {
	t.Parallel()
	command, err := CommandFromPrompt(&mcp.Prompt{
		Name:        "long",
		Description: strings.Repeat("x", 400),
	})
	if err != nil {
		t.Fatalf("CommandFromPrompt: %v", err)
	}
	if got := len([]rune(command.Description)); got != maxCommandDescriptionRunes {
		t.Errorf("description is %d runes, want %d", got, maxCommandDescriptionRunes)
	}
}

// An empty description is refused rather than filled from the name, since a
// command whose description restates its name tells a member nothing.
func TestAPromptWithNoDescriptionIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := CommandFromPrompt(&mcp.Prompt{Name: "bare"}); err == nil {
		t.Error("a prompt with no description produced a command")
	}
}

func TestArgumentsBecomeOptions(t *testing.T) {
	t.Parallel()
	command, err := CommandFromPrompt(&mcp.Prompt{
		Name:        "lookup",
		Description: "look something up",
		Arguments: []*mcp.PromptArgument{
			{Name: "Item Name", Description: "what to look up", Required: true},
			{Name: "store", Required: false},
		},
	})
	if err != nil {
		t.Fatalf("CommandFromPrompt: %v", err)
	}
	if len(command.Options) != 2 {
		t.Fatalf("got %d options, want 2", len(command.Options))
	}
	if command.Options[0].Name != "item-name" || !command.Options[0].Required {
		t.Errorf("first option = %+v", command.Options[0])
	}
	// Discord refuses an empty option description, so the name stands in.
	if command.Options[1].Description == "" {
		t.Error("an option with no description was left empty")
	}
}

// Discord caps options per command, and exceeding it fails the registration
// for the whole set rather than for the one prompt.
func TestTooManyArgumentsAreRefused(t *testing.T) {
	t.Parallel()
	arguments := make([]*mcp.PromptArgument, maxCommandOptions+1)
	for index := range arguments {
		arguments[index] = &mcp.PromptArgument{Name: "a", Description: "d"}
	}
	if _, err := CommandFromPrompt(&mcp.Prompt{
		Name: "wide", Description: "too many", Arguments: arguments,
	}); err == nil {
		t.Error("a prompt over the option ceiling produced a command")
	}
}
