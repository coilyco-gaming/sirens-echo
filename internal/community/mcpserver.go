package community

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpServerPath = "/mcp"

// TurnInput is the MCP tool's argument shape. It mirrors POST /v1/turn, because
// both reach the same turn through a different ingress.
type TurnInput struct {
	Author  string            `json:"author,omitempty"`
	Content string            `json:"content"`
	History []TranscriptEntry `json:"history,omitempty"`
}

// TurnOutput carries the validated reply.
type TurnOutput struct {
	Reply string `json:"reply"`
}

// serverInstructions says what this deployment is, so a client holding several
// servers can tell an agent from a data source. See sirens-echo#647.
func (a *Agent) serverInstructions() string {
	identity := strings.TrimSpace(a.cfg.Definition.Identity)
	if identity == "" {
		identity = "this deployment"
	}
	return fmt.Sprintf(
		"One conversational turn with %s, a Discord community agent. Send a "+
			"message and get the reply it would post: it reads its own "+
			"knowledge and tools, and the answer passes the same response "+
			"checks a member's would. Reach for it to ask a question in this "+
			"community's voice, or to see what it would say. It is an agent "+
			"rather than a data source, so it answers in prose rather than "+
			"records, it may decline, and it is not a passthrough to the "+
			"underlying model.",
		identity,
	)
}

// mcpHandler serves Echo's own turn as an MCP tool, so a fleet client reaches it
// natively instead of learning the bespoke JSON contract.
func (a *Agent) mcpHandler() http.Handler {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    a.cfg.InstanceName,
			Title:   a.cfg.Definition.Identity,
			Version: "1",
		},
		// Only Instructions is set. A nil options pointer is dereferenced into
		// a zero value, so this changes nothing else about the server.
		&mcp.ServerOptions{Instructions: a.serverInstructions()},
	)
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name: "turn",
			Description: "Answer one message through this deployment's " +
				"policy, tools, and response validation.",
		},
		a.handleMCPTurn,
	)
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{},
	)
}

func (a *Agent) handleMCPTurn(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input TurnInput,
) (*mcp.CallToolResult, TurnOutput, error) {
	if strings.TrimSpace(input.Content) == "" {
		return toolFailure("content is required"), TurnOutput{}, nil
	}
	if input.Author == "" {
		input.Author = "mcp client"
	}
	if len([]rune(input.Author)) > 256 || len([]rune(input.Content)) > 16000 {
		return toolFailure("author or content is too long"), TurnOutput{}, nil
	}
	if len(input.History) > a.cfg.Definition.MaxContextMessages {
		return toolFailure("history exceeds the configured context limit"), TurnOutput{}, nil
	}

	// The same admission policy every other ingress uses, so an MCP client
	// cannot outspend the guilds it shares a deployment with.
	decision := a.limiter.Admit(admissionRequest{
		UserKey:    mcpPrincipal(request),
		ContextKey: transportMCP,
		Queued:     true,
	})
	if decision.Outcome.denied() {
		a.telemetry.RecordAdmission(ctx, string(decision.Outcome), transportMCP)
		message := "rate limit reached"
		if decision.RetryAfter > 0 {
			message = fmt.Sprintf(
				"%s, retry after %s seconds",
				message,
				strconv.Itoa(int(math.Ceil(decision.RetryAfter.Seconds()))),
			)
		}
		return toolFailure(message), TurnOutput{}, nil
	}
	defer a.limiter.Release()
	a.telemetry.RecordAdmission(ctx, string(admissionAccepted), transportMCP)

	turn := &httpTurn{
		requestID: fmt.Sprintf("mcp-%d", time.Now().UnixNano()),
		requester: mcpPrincipal(request),
		transport: transportMCP,
		history:   assertedHistory(input.History),
		current:   TranscriptEntry{Author: input.Author, Content: input.Content},
	}
	if err := a.runSerialized(ctx, turn, transportMCP); err != nil {
		// The reply is the runtime's own failure text, which is safe to return.
		return toolFailure(turn.reply), TurnOutput{}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: turn.reply}},
	}, TurnOutput{Reply: turn.reply}, nil
}

// toolFailure reports a caller-fixable problem as tool data rather than a
// protocol error, so the calling model can see it and correct itself.
func toolFailure(message string) *mcp.CallToolResult {
	if strings.TrimSpace(message) == "" {
		message = noticeTurnFailed
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: message}},
	}
}

// mcpPrincipal names the per-caller admission budget. The shared header wins so
// one client can separate its own callers, then the declared client name.
func mcpPrincipal(request *mcp.CallToolRequest) string {
	if request != nil && request.Extra != nil && request.Extra.Header != nil {
		if caller := strings.TrimSpace(request.Extra.Header.Get("X-Sirens-Caller")); caller != "" {
			return "mcp:" + cleanTranscriptText(caller, 64)
		}
	}
	if request != nil && request.Session != nil {
		if params := request.Session.InitializeParams(); params != nil &&
			params.ClientInfo != nil &&
			strings.TrimSpace(params.ClientInfo.Name) != "" {
			return "mcp:" + cleanTranscriptText(params.ClientInfo.Name, 64)
		}
	}
	return "mcp:anonymous"
}
