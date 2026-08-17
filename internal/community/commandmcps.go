package community

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// ephemeralFlag answers the caller alone when the command declared it. Zero is
// Discord's ordinary visible reply, so a command that declares nothing is.
func ephemeralFlag(ephemeral bool) discordgo.MessageFlags {
	if ephemeral {
		return discordgo.MessageFlagsEphemeral
	}
	return 0
}

// /mcps reports the deployment's own tool surface. Names only: an address is an
// identifier. See docs/sirens-echo-commands.md.

// renderMCPRoster groups discovered tools by server, naming every configured
// one so a silent server is reported. See docs/sirens-echo-commands.md.
func renderMCPRoster(configured []MCPServerDefinition, tools []ToolDefinition, unavailable []string) string {
	if len(configured) == 0 {
		return "no MCP server is configured for this deployment"
	}
	byServer := make(map[string][]string, len(configured))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Original)
		if name == "" {
			name = tool.Name
		}
		byServer[tool.Server] = append(byServer[tool.Server], name)
	}
	down := make(map[string]bool, len(unavailable))
	for _, name := range unavailable {
		down[name] = true
	}

	lines := make([]string, 0, len(configured))
	for _, server := range configured {
		names := byServer[server.Name]
		sort.Strings(names)
		switch {
		case down[server.Name]:
			// Distinguished from "answered with nothing", because an operator
			// reading this needs to know which one happened.
			lines = append(lines, fmt.Sprintf("%s: did not answer this turn", server.Name))
		case len(names) == 0:
			lines = append(lines, fmt.Sprintf("%s: no tools", server.Name))
		default:
			lines = append(lines, fmt.Sprintf("%s (%d): %s",
				server.Name, len(names), strings.Join(names, ", ")))
		}
	}
	return boundMCPRoster(lines)
}

// boundMCPRoster keeps the reply inside the interaction bound and says when it
// dropped something, since a silently short list reads as a short roster.
func boundMCPRoster(lines []string) string {
	rendered := strings.Join(lines, "\n")
	if len(rendered) <= mcpsReplyBudget {
		return rendered
	}
	kept := make([]string, 0, len(lines))
	used := 0
	for _, line := range lines {
		if used+len(line)+1 > mcpsReplyBudget {
			break
		}
		kept = append(kept, line)
		used += len(line) + 1
	}
	dropped := len(lines) - len(kept)
	return fmt.Sprintf("%s\n(%d more server(s) not shown: the roster exceeds one reply)",
		strings.Join(kept, "\n"), dropped)
}

// mcpRoster opens the same session a turn would, so the answer is what this
// process can actually reach rather than what the file declares.
func (a *Agent) mcpRoster(ctx context.Context) string {
	if a.tools == nil {
		return "no MCP server is configured for this deployment"
	}
	session, err := a.tools.Open(ctx)
	if err != nil {
		return harnessNotice("the tool roster could not be read")
	}
	return renderMCPRoster(a.tools.Servers, session.Tools(), session.Unavailable())
}
