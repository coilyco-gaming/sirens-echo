package community

import (
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community/systemone"
)

// The tool family picks a server, then its tool, as Jev choices: noul values are
// not calibrated confidences (sirens-echo#8229). See docs/sirens-echo-tools.md.
const (
	// jevToolThreshold is Kai's bar, applied to the server pick and the tool pick.
	jevToolThreshold = 0.9
	// jevToolMaxOptions is the choice size Jev is reliable at, per the
	// tooling-jev-decisions skill. A larger server is not asked about.
	jevToolMaxOptions = 240

	toolServerKey       = "tool.server"
	toolPickKeyPrefix   = "tool.pick:"
	toolServerNone      = "none"
	toolNoToolOption    = "no_tool"
	toolServerNoneText  = "No server's tools are needed: social talk, opinions, how-to answerable from knowledge, or a request addressed to a person."
	toolNoToolText      = "No tool from this server answers this. Game mechanics and how-to, client crashes and bug reports, wipe or patch schedules, requests addressed to a specific person, and anything outside this server's data."
	toolServerFallback  = "Tools from the MCP server "
	jevFallbackNoListed = jevFallbackReason("no_tool_listing")
)

// CachedServerTools is one server's last listing, read without dialing.
type CachedServerTools struct {
	Server   string
	Guidance string
	Tools    []*mcp.Tool
}

// CachedTools snapshots every listed server. A server not yet listed is absent,
// so the first turn after boot asks nothing rather than waiting on a dial.
func (p *MCPProvider) CachedTools() []CachedServerTools {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]CachedServerTools, 0, len(p.entries))
	for _, entry := range p.entries {
		if len(entry.tools) == 0 {
			continue
		}
		listing := CachedServerTools{
			Server: entry.definition.Name,
			Tools:  append([]*mcp.Tool(nil), entry.tools...),
		}
		if guidance, ok := serverGuidance(entry.definition.Name, entry.session); ok {
			listing.Guidance = guidance.Text
		}
		out = append(out, listing)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Server < out[j].Server })
	return out
}

// toolRouteQuestions builds the server pick and one tool pick per server.
func toolRouteQuestions(listings []CachedServerTools) ([]systemone.Question, map[string]jevQuestionMeta) {
	questions := make([]systemone.Question, 0, len(listings)+1)
	meta := make(map[string]jevQuestionMeta)
	servers := []systemone.Criterion{{Name: toolServerNone, Description: toolServerNoneText}}
	for _, listing := range listings {
		if len(listing.Tools)+1 > jevToolMaxOptions {
			continue
		}
		criteria := make([]systemone.Criterion, 0, len(listing.Tools)+1)
		for _, tool := range listing.Tools {
			if tool == nil || tool.Name == "" || tool.Name == toolNoToolOption {
				continue
			}
			criteria = append(criteria, systemone.Criterion{Name: tool.Name, Description: tool.Description})
		}
		if len(criteria) == 0 {
			continue
		}
		criteria = append(criteria, systemone.Criterion{Name: toolNoToolOption, Description: toolNoToolText})
		description := listing.Guidance
		if description == "" {
			description = toolServerFallback + "\"" + listing.Server + "\"."
		}
		servers = append(servers, systemone.Criterion{Name: listing.Server, Description: description})
		key := toolPickKeyPrefix + listing.Server
		questions = append(questions, systemone.Question{
			Key:  key,
			Type: systemone.TypeChoice,
			Prompt: "Which single tool from the MCP server \"" + listing.Server +
				"\" best answers the member's message? Pick " + toolNoToolOption +
				" when no tool's data can answer it.",
			Criteria: criteria,
		})
		meta[key] = jevQuestionMeta{family: RouteFamilyTool, target: listing.Server}
	}
	if len(servers) == 1 {
		return nil, nil
	}
	questions = append([]systemone.Question{{
		Key:      toolServerKey,
		Type:     systemone.TypeChoice,
		Prompt:   "Which MCP server's tools, if any, would answer the member's message?",
		Criteria: servers,
	}}, questions...)
	meta[toolServerKey] = jevQuestionMeta{family: RouteFamilyTool}
	return questions, meta
}

// applyToolAnswers reads the server pick, then that server's tool pick. A
// "none" or "no_tool" winner is a decline, recorded with its probability.
func applyToolAnswers(decision *RouteDecision, meta map[string]jevQuestionMeta, answers map[string]systemone.Answer) {
	if _, asked := meta[toolServerKey]; !asked {
		decision.fellBackTo(RouteFamilyTool, jevFallbackNoListed)
		return
	}
	server, ok := answers[toolServerKey]
	if !ok {
		decision.fellBackTo(RouteFamilyTool, jevFallbackMissingAnswer)
		return
	}
	decision.ToolServerProb = server.Probability
	if server.Option == toolServerNone {
		return
	}
	decision.ToolServer = server.Option
	pick, ok := answers[toolPickKeyPrefix+server.Option]
	if !ok {
		decision.fellBackTo(RouteFamilyTool, jevFallbackMissingAnswer)
		return
	}
	decision.ToolProb = pick.Probability
	if pick.Option == toolNoToolOption {
		return
	}
	decision.Tool = pick.Option
}

// DirectTool is the server and tool to call without the model: both picks at
// or above the bar, never on a fallback. Stage 1 only traces it.
func (d RouteDecision) DirectTool() (server, tool string, ok bool) {
	if !d.Ran || d.hasFallback(RouteFamilyTool) || d.Tool == "" {
		return "", "", false
	}
	if d.ToolServerProb < jevToolThreshold || d.ToolProb < jevToolThreshold {
		return "", "", false
	}
	return d.ToolServer, d.Tool, true
}
