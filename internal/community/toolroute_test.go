package community

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"forgejo.coilysiren.me/coilyco-gaming/sirens-echo/internal/community/systemone"
)

func ecoListing() CachedServerTools {
	return CachedServerTools{
		Server: "eco",
		Tools: []*mcp.Tool{
			{Name: "find_trade", Description: "Where to buy or sell an item."},
			{Name: "get_market", Description: "How an item's price has moved."},
		},
	}
}

func TestToolRouteQuestionsAskServerThenOneToolChoicePerServer(t *testing.T) {
	questions, meta := toolRouteQuestions([]CachedServerTools{ecoListing()})

	if len(questions) != 2 {
		t.Fatalf("questions = %d, want the server pick plus one tool pick", len(questions))
	}
	server := questions[0]
	if server.Key != toolServerKey || server.Type != systemone.TypeChoice {
		t.Fatalf("first question = %s/%s, want the %s choice", server.Key, server.Type, toolServerKey)
	}
	if names := criterionNames(server.Criteria); fmt.Sprint(names) != "[none eco]" {
		t.Errorf("server options = %v, want [none eco]", names)
	}
	pick := questions[1]
	if pick.Key != toolPickKeyPrefix+"eco" || pick.Type != systemone.TypeChoice {
		t.Fatalf("second question = %s/%s, want the eco tool choice", pick.Key, pick.Type)
	}
	if names := criterionNames(pick.Criteria); fmt.Sprint(names) != "[find_trade get_market no_tool]" {
		t.Errorf("tool options = %v, want the listed tools plus no_tool", names)
	}
	if meta[pick.Key].target != "eco" || meta[pick.Key].family != RouteFamilyTool {
		t.Errorf("meta for %s = %+v, want the tool family targeting eco", pick.Key, meta[pick.Key])
	}
}

func TestToolRouteQuestionsSkipAServerPastJevsChoiceSize(t *testing.T) {
	big := CachedServerTools{Server: "huge"}
	for i := range jevToolMaxOptions {
		big.Tools = append(big.Tools, &mcp.Tool{Name: fmt.Sprintf("t%d", i)})
	}
	questions, _ := toolRouteQuestions([]CachedServerTools{big, ecoListing()})

	for _, q := range questions {
		if q.Key == toolPickKeyPrefix+"huge" {
			t.Fatal("asked about a server whose tools plus no_tool exceed the choice size")
		}
	}
	if names := criterionNames(questions[0].Criteria); fmt.Sprint(names) != "[none eco]" {
		t.Errorf("server options = %v, want the oversize server left out", names)
	}
}

func TestToolRouteQuestionsAskNothingWithoutAListing(t *testing.T) {
	if questions, _ := toolRouteQuestions(nil); len(questions) != 0 {
		t.Fatalf("questions = %d with no cached listing, want none", len(questions))
	}
}

func TestApplyToolAnswersCallsDirectlyOnlyWhenBothPicksClearTheBar(t *testing.T) {
	_, meta := toolRouteQuestions([]CachedServerTools{ecoListing()})
	cases := []struct {
		name       string
		serverProb float64
		toolOption string
		toolProb   float64
		wantDirect bool
	}{
		{"both clear", 0.95, "find_trade", 0.93, true},
		{"server under the bar", 0.85, "find_trade", 0.99, false},
		{"tool under the bar", 0.97, "find_trade", 0.62, false},
		{"no_tool wins", 0.97, toolNoToolOption, 0.95, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := RouteDecision{Ran: true}
			applyToolAnswers(&decision, meta, map[string]systemone.Answer{
				toolServerKey:             {Key: toolServerKey, Option: "eco", Probability: tc.serverProb},
				toolPickKeyPrefix + "eco": {Key: toolPickKeyPrefix + "eco", Option: tc.toolOption, Probability: tc.toolProb},
			})
			server, tool, direct := decision.DirectTool()
			if direct != tc.wantDirect {
				t.Fatalf("direct = %v (%s/%s), want %v", direct, server, tool, tc.wantDirect)
			}
		})
	}
}

func TestApplyToolAnswersRecordsADeclineAndEachFallback(t *testing.T) {
	_, meta := toolRouteQuestions([]CachedServerTools{ecoListing()})

	declined := RouteDecision{Ran: true}
	applyToolAnswers(&declined, meta, map[string]systemone.Answer{
		toolServerKey: {Key: toolServerKey, Option: toolServerNone, Probability: 0.97},
	})
	if declined.ToolServer != "" || declined.hasFallback(RouteFamilyTool) {
		t.Errorf("none winner: server %q fallback %v, want an empty pick and no fallback", declined.ToolServer, declined.Fallbacks)
	}

	missing := RouteDecision{Ran: true}
	applyToolAnswers(&missing, meta, map[string]systemone.Answer{
		toolServerKey: {Key: toolServerKey, Option: "eco", Probability: 0.97},
	})
	if missing.Fallbacks[RouteFamilyTool] != jevFallbackMissingAnswer {
		t.Errorf("missing tool pick fallback = %q, want %q", missing.Fallbacks[RouteFamilyTool], jevFallbackMissingAnswer)
	}

	unlisted := RouteDecision{Ran: true}
	applyToolAnswers(&unlisted, nil, nil)
	if unlisted.Fallbacks[RouteFamilyTool] != jevFallbackNoListed {
		t.Errorf("no listing fallback = %q, want %q", unlisted.Fallbacks[RouteFamilyTool], jevFallbackNoListed)
	}
}

func TestCachedToolsReadsListedServersWithoutDialing(t *testing.T) {
	provider := &MCPProvider{entries: []*supervisedServer{
		{definition: MCPServerDefinition{Name: "unlisted"}},
		{definition: MCPServerDefinition{Name: "eco"}, tools: ecoListing().Tools},
	}}

	listings := provider.CachedTools()

	if len(listings) != 1 || listings[0].Server != "eco" || len(listings[0].Tools) != 2 {
		t.Fatalf("listings = %+v, want only eco with its two tools", listings)
	}
	if listings[0].Guidance != "" {
		t.Errorf("guidance = %q with no session, want empty so the fallback text is used", listings[0].Guidance)
	}
}

func TestRouteJevTracesADirectToolPick(t *testing.T) {
	agent := testJevAgent(t)
	agent.cfg.JevModel = "jev-latest"
	agent.tools = &MCPProvider{entries: []*supervisedServer{
		{definition: MCPServerDefinition{Name: "eco"}, tools: ecoListing().Tools},
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req systemone.Request
		decodeJSON(t, r, &req)
		resp := systemone.Response{}
		for _, q := range req.Questions {
			switch q.Key {
			case toolServerKey:
				resp.Answers = append(resp.Answers, systemone.Answer{Key: q.Key, Option: "eco", Probability: 0.96})
			case toolPickKeyPrefix + "eco":
				resp.Answers = append(resp.Answers, systemone.Answer{Key: q.Key, Option: "find_trade", Probability: 0.94})
			default:
				resp.Answers = append(resp.Answers, systemone.Answer{Key: q.Key, Probability: 0.1})
			}
		}
		encodeJSON(t, w, resp)
	}))
	defer server.Close()
	agent.cfg.AgentProxyURL = server.URL

	decision := agent.routeJev(context.Background(), TranscriptEntry{Content: "where can I buy limestone"}, "req-tool", false, false)

	serverName, tool, direct := decision.DirectTool()
	if !direct || serverName != "eco" || tool != "find_trade" {
		t.Fatalf("DirectTool = %s/%s/%v, want eco/find_trade/true", serverName, tool, direct)
	}
}

func TestJevDisableToolAsksNoToolQuestions(t *testing.T) {
	agent := testJevAgent(t)
	agent.tools = &MCPProvider{entries: []*supervisedServer{
		{definition: MCPServerDefinition{Name: "eco"}, tools: ecoListing().Tools},
	}}

	questions, _ := agent.buildRouteQuestions(TranscriptEntry{Content: "hi"}, jevDisabledSet([]string{"tool"}), false, false)

	for _, q := range questions {
		if q.Key == toolServerKey || q.Key == toolPickKeyPrefix+"eco" {
			t.Fatalf("asked %s with the tool family disabled", q.Key)
		}
	}
}

func criterionNames(criteria []systemone.Criterion) []string {
	names := make([]string, 0, len(criteria))
	for _, c := range criteria {
		names = append(names, c.Name)
	}
	return names
}
