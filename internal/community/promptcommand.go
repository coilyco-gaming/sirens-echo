package community

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A server prompt is user-selected instruction reaching the model through a
// structured channel. See docs/sirens-echo-prompt-commands.md.

const (
	// Discord's limits. A breach fails the whole registration, so one
	// server's prompt would otherwise cost every command.
	maxCommandNameRunes        = 32
	maxCommandDescriptionRunes = 100
	// maxCommandOptions is Discord's ceiling on options per command.
	maxCommandOptions = 25
)

// discordCommandName is the shape Discord accepts: lower case, no spaces.
var discordCommandName = regexp.MustCompile(`^[-_a-z0-9]{1,32}$`)

// commandNameFor lowercases and replaces what Discord refuses. An MCP prompt
// name is server-supplied, so it satisfies none of this by construction.
func commandNameFor(prompt string) string {
	lowered := strings.ToLower(strings.TrimSpace(prompt))
	cleaned := strings.Map(func(current rune) rune {
		switch {
		case current >= 'a' && current <= 'z', current >= '0' && current <= '9':
			return current
		case current == '-', current == '_':
			return current
		case current == ' ', current == '.', current == '/', current == ':':
			return '-'
		default:
			return -1
		}
	}, lowered)
	cleaned = strings.Trim(cleaned, "-_")
	if len([]rune(cleaned)) > maxCommandNameRunes {
		cleaned = string([]rune(cleaned)[:maxCommandNameRunes])
		cleaned = strings.Trim(cleaned, "-_")
	}
	return cleaned
}

// truncateDescription keeps Discord's limit without cutting a rune in half. An
// empty description is refused rather than filled with the name.
func truncateDescription(description string) string {
	trimmed := strings.Join(strings.Fields(description), " ")
	runes := []rune(trimmed)
	if len(runes) <= maxCommandDescriptionRunes {
		return trimmed
	}
	return string(runes[:maxCommandDescriptionRunes])
}

// CommandFromPrompt renders one server prompt as a Discord command. It refuses
// rather than repairs, because a malformed set costs every command in it.
func CommandFromPrompt(prompt *mcp.Prompt) (*discordgo.ApplicationCommand, error) {
	if prompt == nil {
		return nil, fmt.Errorf("no prompt")
	}
	name := commandNameFor(prompt.Name)
	if !discordCommandName.MatchString(name) {
		return nil, fmt.Errorf("prompt %q has no usable command name", prompt.Name)
	}
	description := truncateDescription(prompt.Description)
	if description == "" {
		return nil, fmt.Errorf("prompt %q has no description", prompt.Name)
	}
	if len(prompt.Arguments) > maxCommandOptions {
		return nil, fmt.Errorf(
			"prompt %q declares %d arguments against a ceiling of %d",
			prompt.Name, len(prompt.Arguments), maxCommandOptions,
		)
	}
	options := make([]*discordgo.ApplicationCommandOption, 0, len(prompt.Arguments))
	for _, argument := range prompt.Arguments {
		if argument == nil {
			continue
		}
		optionName := commandNameFor(argument.Name)
		if !discordCommandName.MatchString(optionName) {
			return nil, fmt.Errorf(
				"prompt %q argument %q has no usable option name", prompt.Name, argument.Name,
			)
		}
		optionDescription := truncateDescription(argument.Description)
		if optionDescription == "" {
			// Discord refuses an empty option description, and the argument name
			// is the only thing always present to stand in for it.
			optionDescription = optionName
		}
		options = append(options, &discordgo.ApplicationCommandOption{
			Name:        optionName,
			Description: optionDescription,
			Required:    argument.Required,
			Type:        discordgo.ApplicationCommandOptionString,
		})
	}
	return &discordgo.ApplicationCommand{
		Name:        name,
		Description: description,
		Options:     options,
	}, nil
}
