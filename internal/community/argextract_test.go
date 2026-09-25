package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func itemVocab() []VocabEntry {
	return []VocabEntry{
		{ID: "LimestoneItem", Name: "Limestone"},
		{ID: "CrushedLimestoneItem", Name: "Crushed Limestone"},
		{ID: "HewnLogItem", Name: "Hewn Log", Aliases: []string{"hewn logs"}},
		{ID: "IronBarItem", Name: "Iron Bar"},
		{ID: "IronOreItem", Name: "Iron Ore"},
	}
}

func TestMatchVocabPicksTheLongestWholeWordForm(t *testing.T) {
	cases := map[string]string{
		"where can I buy limestone":                      "LimestoneItem",
		"who sells crushed limestone cheapest":           "CrushedLimestoneItem",
		"what are the prices of hewn logs?":              "HewnLogItem",
		"who is selling iron bars cheapest right now":    "IronBarItem",
		"Where can I buy LIMESTONE, asking for a friend": "LimestoneItem",
	}
	for message, want := range cases {
		got, ok := matchVocab(message, itemVocab())
		if !ok || got.ID != want {
			t.Errorf("%q: got %q/%v, want %q", message, got.ID, ok, want)
		}
	}
}

func TestMatchVocabDeclinesOnNoMatchPartialWordsOrATie(t *testing.T) {
	cases := []string{
		"is the server up",           // nothing named
		"limestones are overrated??", // "limestones" matches, so this one is a positive control below
		"ironbar prices",             // not whole words of "iron bar"
		"iron bar or iron ore",       // two entries tie at two words
	}
	if _, ok := matchVocab(cases[0], itemVocab()); ok {
		t.Errorf("%q matched, want nothing named", cases[0])
	}
	if got, ok := matchVocab(cases[1], itemVocab()); !ok || got.ID != "LimestoneItem" {
		t.Errorf("%q: got %q/%v, want the plural to match", cases[1], got.ID, ok)
	}
	for _, message := range cases[2:] {
		if got, ok := matchVocab(message, itemVocab()); ok {
			t.Errorf("%q matched %q, want a decline", message, got.ID)
		}
	}
}

func TestToolArgSpecsSkipsMalformedDeclarations(t *testing.T) {
	tool := &mcp.Tool{Name: "find_trade"}
	tool.Meta = mcp.Meta{toolArgsMetaKey: map[string]any{
		"item":     map[string]any{"vocabulary": "eco://vocab/items", "field": "name"},
		"product":  map[string]any{"vocabulary": "eco://vocab/items", "field": "slug"},
		"currency": "not an object",
	}}

	specs := toolArgSpecs(tool)

	if len(specs) != 1 || specs["item"].Vocabulary != "eco://vocab/items" || specs["item"].Field != "name" {
		t.Fatalf("specs = %+v, want only the well-formed item declaration", specs)
	}
}

// tradeServer publishes find_trade with an item-gated template, an args
// declaration, and the vocabulary resource it points at, as eco-app does.
func tradeServer(t *testing.T, reads *atomic.Int32) *MCPProvider {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "eco-test", Version: "1"}, nil)
	body, err := json.Marshal(map[string]any{"entries": itemVocab()})
	if err != nil {
		t.Fatal(err)
	}
	server.AddResource(
		&mcp.Resource{URI: "eco://vocab/items", Name: "items", MIMEType: "application/json"},
		func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			reads.Add(1)
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: "eco://vocab/items", MIMEType: "application/json", Text: string(body)}},
			}, nil
		},
	)
	tool := &mcp.Tool{Name: "find_trade", Description: "trade", InputSchema: map[string]any{"type": "object"}}
	tool.Meta = mcp.Meta{
		replyTemplatesMetaKey: []any{map[string]any{
			"when_args": []any{"item"},
			"text":      "The cheapest {{args.item}} is {{cheapest.0.price}} Credits.",
		}},
		toolArgsMetaKey: map[string]any{"item": map[string]any{"vocabulary": "eco://vocab/items", "field": "name"}},
	}
	server.AddTool(tool, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		_ = json.Unmarshal(req.Params.Arguments, &args)
		if args["item"] != "Limestone" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "no match"}}, StructuredContent: map[string]any{"cheapest": []any{}}}, nil
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: "summary"}},
			StructuredContent: map[string]any{"cheapest": []any{map[string]any{"price": 3}}},
		}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	t.Cleanup(httpServer.Close)
	provider := &MCPProvider{Servers: []MCPServerDefinition{{Name: "eco", URL: httpServer.URL}}}
	t.Cleanup(func() { _ = provider.Close() })
	session, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = session.Close()
	return provider
}

func confidentTradePick() RouteDecision {
	return RouteDecision{Ran: true, ToolServer: "eco", Tool: "find_trade", ToolProb: 0.96}
}

func TestDirectToolReplyFillsTheItemFromTheServersVocabulary(t *testing.T) {
	var reads atomic.Int32
	agent := testJevAgent(t)
	agent.cfg.JevDirectTools = true
	agent.tools = tradeServer(t, &reads)

	got, ok := agent.directToolReply(context.Background(), confidentTradePick(), "where can I buy limestone")
	if !ok || got != "The cheapest Limestone is 3 Credits." {
		t.Fatalf("directToolReply = %q, %v, want the item filled from the vocabulary", got, ok)
	}
	if _, ok := agent.directToolReply(context.Background(), confidentTradePick(), "where can I buy limestone"); !ok {
		t.Fatal("second identical turn declined, want the cached vocabulary to answer again")
	}
	if reads.Load() != 1 {
		t.Errorf("vocabulary reads = %d over two turns, want 1 from the cache", reads.Load())
	}
}

func TestDirectToolReplyDeclinesWhenNoItemIsNamed(t *testing.T) {
	var reads atomic.Int32
	agent := testJevAgent(t)
	agent.cfg.JevDirectTools = true
	agent.tools = tradeServer(t, &reads)

	if got, ok := agent.directToolReply(context.Background(), confidentTradePick(), "where can I buy stuff"); ok {
		t.Fatalf("directToolReply = %q, want a decline so the model path runs", got)
	}
}
