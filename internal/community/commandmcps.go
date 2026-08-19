package community

import (
	"context"
	"fmt"
	"slices"
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

// boundMCPRoster keeps either reply inside the interaction bound and says when
// it dropped something, since a silently short list reads as a short one.
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
	return fmt.Sprintf("%s\n(%d more line(s) not shown: this exceeds one reply)",
		strings.Join(kept, "\n"), dropped)
}

// mcpServerNames is the configured roster, which closes /mcp's argument and is
// read at registration and at dispatch alike.
func (a *Agent) mcpServerNames() []string {
	if a.tools == nil {
		return nil
	}
	names := make([]string, 0, len(a.tools.Servers))
	for _, server := range a.tools.Servers {
		if server.Name != "" {
			names = append(names, server.Name)
		}
	}
	sort.Strings(names)
	return names
}

// mcpServer describes one server's tools. It exists because /mcps is an index
// bounded by one reply, and a tool's description never fits in an index.
func (a *Agent) mcpServer(ctx context.Context, name string) string {
	if a.tools == nil {
		return "no MCP server is configured for this deployment"
	}
	// Re-checked rather than trusted to the picker, because a stale published
	// choice outlives the roster it was rendered from.
	if !slices.Contains(a.mcpServerNames(), name) {
		return harnessNotice("no server by that name is configured")
	}
	session, err := a.tools.Open(ctx)
	if err != nil {
		return harnessNotice("the tool roster could not be read")
	}
	return renderMCPServer(name, session.Tools(), session.Unavailable())
}

// renderMCPServer lists one server's tools with what each one does, which is
// the half /mcps has no room for. See docs/sirens-echo-commands.md.
func renderMCPServer(name string, tools []ToolDefinition, unavailable []string) string {
	if slices.Contains(unavailable, name) {
		return fmt.Sprintf("%s: did not answer this turn", name)
	}
	lines := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.Server != name {
			continue
		}
		lines = append(lines, mcpToolLine(tool))
	}
	if len(lines) == 0 {
		return fmt.Sprintf("%s: no tools", name)
	}
	sort.Strings(lines)
	return boundMCPRoster(append(
		[]string{fmt.Sprintf("%s (%d):", name, len(lines))}, lines...))
}

// mcpToolLine renders one tool. The description is the server's own text, so it
// is bounded and flattened before it reaches a reply.
func mcpToolLine(tool ToolDefinition) string {
	name := strings.TrimSpace(tool.Original)
	if name == "" {
		name = tool.Name
	}
	summary := strings.Join(strings.Fields(tool.Description), " ")
	if summary == "" {
		return "- " + name
	}
	return fmt.Sprintf("- %s: %s", name, hardTrimRunes(summary, mcpToolSummaryRunes))
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
