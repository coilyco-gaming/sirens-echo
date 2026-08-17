package community

import "testing"

// The patterns were widened against 396 persisted replies. These rows are the
// evidence, not the corpus. See docs/sirens-echo-tool-markup.md.

// observedMarkupShapes are replies a live model actually produced. The last two
// are the tool-name-as-tag family a closed set of published names cannot cover.
var observedMarkupShapes = map[string]string{
	"deepseek delimiters": "Checking now.\n<｜｜DSML｜｜tool_calls>\n" +
		"<｜｜DSML｜｜invoke name=\"search_issues\">",
	"published call tag": "I'll look.\n<tool_call>{\"name\": \"list_issue\"}</tool_call>",
	"control token":      "One moment.<|python_tag|>",
	"wrapper the published names miss": "<mm_tool_calls>\n" +
		"    <mm_tool_call name=\"forgejo_search_issues\">\n" +
		"        <parameters>{\"q\": \"stale server status\"}</parameters>\n" +
		"    </mm_tool_call>\n</mm_tool_calls>",
	"tag named after the tool":   "<create_issue name=\"file a correction\">",
	"a round rather than a call": "<tool_round>\n{\"name\": \"get_server_status\"}\n</tool_round>",
}

func TestTheMarkupCheckCatchesEveryObservedShape(t *testing.T) {
	t.Parallel()
	for name, reply := range observedMarkupShapes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := checkToolCallMarkup(reply); err == nil {
				t.Error("a shape a live model produced scored as a clean reply, so a " +
					"member would read the delimiters verbatim")
			}
		})
	}
}

// The rows that matter more. Each is a real reply, or the shape of one, that
// must stay legal. A widened pattern eating these costs a member their answer.
var cleanReplyShapes = map[string]string{
	"names a tool it cannot call": "This service cannot call list_issue, " +
		"since that tool is not in its roster.",
	"describes the harness": "The harness emits tool_calls as a structured field " +
		"rather than as reply content.",
	"quotes a proxy log field": "The proxy log line carries \"tool_calls\": [...] " +
		"when a call was made.",
	"an ordinary refusal": "Nothing in this turn returned a tracker result, so no " +
		"issue can be named from here.",
	"markdown with an attribute-like phrase": "Use the wiki page name = Algebra " +
		"when searching, and the search box will find it.",
	"an angle-bracket comparison": "A reply under 200 words is fine; anything " +
		"over 1800 characters is rejected.",
	"prose naming a parameter": "The limit parameter caps a listing at 25 entries.",
}

func TestTheWidenedPatternsStayOffCorrectReplies(t *testing.T) {
	t.Parallel()
	for name, reply := range cleanReplyShapes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := checkToolCallMarkup(reply); err != nil {
				t.Errorf("a correct reply was scored as markup: %v", err)
			}
		})
	}
}
