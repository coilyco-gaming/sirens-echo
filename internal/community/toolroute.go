package community

import (
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community/systemone"
)

// The tool family asks one Jev choice per server. The most confident non-no_tool pick
// names the server, since a separate server pick declined data (sirens-echo#8229).
const (
	// jevToolThreshold is Kai's bar, applied to the winning tool pick.
	jevToolThreshold = 0.9
	// jevToolContested is where a second server's tool pick blocks a direct call.
	jevToolContested = 0.5
	// jevToolMaxOptions is the choice size Jev is reliable at, per the
	// tooling-jev-decisions skill. A larger server is not asked about.
	jevToolMaxOptions = 240

	toolPickKeyPrefix   = "tool.pick:"
	toolNoToolOption    = "no_tool"
	toolNoToolText      = "No tool from this server answers this. Game mechanics and how-to, client crashes and bug reports, wipe or patch schedules, requests addressed to a specific person, and anything outside this server's data."
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

// toolRouteQuestions builds one tool pick per listed server.
func toolRouteQuestions(listings []CachedServerTools) ([]systemone.Question, map[string]jevQuestionMeta) {
	questions := make([]systemone.Question, 0, len(listings))
	meta := make(map[string]jevQuestionMeta)
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
		prompt := "Which single tool from the MCP server \"" + listing.Server +
			"\" best answers the member's message? Pick " + toolNoToolOption +
			" when no tool's data can answer it."
		if listing.Guidance != "" {
			prompt += " The server describes itself: " + listing.Guidance
		}
		key := toolPickKeyPrefix + listing.Server
		questions = append(questions, systemone.Question{
			Key:      key,
			Type:     systemone.TypeChoice,
			Prompt:   prompt,
			Criteria: criteria,
		})
		meta[key] = jevQuestionMeta{family: RouteFamilyTool, target: listing.Server}
	}
	return questions, meta
}

// applyToolAnswers takes the most confident non-no_tool pick across servers.
// ToolServerProb carries the strongest rival server's pick, which contests it.
func applyToolAnswers(decision *RouteDecision, meta map[string]jevQuestionMeta, answers map[string]systemone.Answer) {
	asked, answered := false, false
	for key, m := range meta {
		if m.family != RouteFamilyTool {
			continue
		}
		asked = true
		pick, ok := answers[key]
		if !ok {
			continue
		}
		answered = true
		if pick.Option == toolNoToolOption || pick.Option == "" {
			continue
		}
		if pick.Probability > decision.ToolProb {
			decision.ToolServerProb = max(decision.ToolServerProb, decision.ToolProb)
			decision.ToolServer, decision.Tool, decision.ToolProb = m.target, pick.Option, pick.Probability
		} else {
			decision.ToolServerProb = max(decision.ToolServerProb, pick.Probability)
		}
	}
	switch {
	case !asked:
		decision.fellBackTo(RouteFamilyTool, jevFallbackNoListed)
	case !answered:
		decision.fellBackTo(RouteFamilyTool, jevFallbackMissingAnswer)
	}
}

// DirectTool is the server and tool to call without the model: the winning pick at
// the bar, no rival server's pick at jevToolContested, never on a fallback.
func (d RouteDecision) DirectTool() (server, tool string, ok bool) {
	if !d.Ran || d.hasFallback(RouteFamilyTool) || d.Tool == "" {
		return "", "", false
	}
	if d.ToolProb < jevToolThreshold || d.ToolServerProb >= jevToolContested {
		return "", "", false
	}
	return d.ToolServer, d.Tool, true
}
