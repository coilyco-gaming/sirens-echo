package community

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type exceptionCode uint8

const (
	exceptionUnclassified exceptionCode = iota
	exceptionTurnFailed
	exceptionHistoryFailed
	exceptionResponseValidationFailed
	exceptionDiscordReplyFailed
	exceptionReplyFailed
	exceptionMCPToolsListFailed
	exceptionMCPServerUnavailable
	exceptionMCPSessionCloseFailed
	exceptionMCPToolCallFailed
	exceptionModelRequestMarshalFailed
	exceptionModelRequestBuildFailed
	exceptionModelTransportFailed
	exceptionModelResponseReadFailed
	exceptionModelResponseTooLarge
	exceptionModelResponseHTTPError
	exceptionModelResponseDecodeFailed
	exceptionModelResponseMissingChoice
	exceptionModelSilent
	exceptionHTTPTurnMethodNotAllowed
	exceptionHTTPTurnInvalidJSON
	exceptionHTTPTurnBodyTooLarge
	exceptionHTTPTurnUnknownField
	exceptionHTTPTurnContentRequired
	exceptionHTTPTurnInputTooLong
	exceptionHTTPTurnHistoryTooLong
	exceptionHTTPTurnPromptFailed
	exceptionHTTPTurnRateLimited
	exceptionJobRequestInvalid
	exceptionJobBodyTooLarge
	exceptionJobRejected
	exceptionJobNotPermitted
	exceptionJobNotFound
	exceptionJobQueueFull
	exceptionCommandFailed
	exceptionAttachmentFetchFailed
	exceptionContentGateModelFailed
	exceptionContentGateUnknownClass
	exceptionCodeCount
)

// The fault values. A string rather than a bool, so a code that declares
// nothing is empty rather than silently reading as the service's fault.
const (
	faultCaller  = "caller"
	faultService = "service"
)

type exceptionSpec struct {
	typeName string
	message  string
	stage    string
	outcome  string
	// fault separates a caller mistake from a service failure. The stage cannot
	// do it: prompt_failed is an MCP failure on the HTTP path. See issue 159.
	fault string
}

var exceptionCatalog = [exceptionCodeCount]exceptionSpec{
	exceptionUnclassified: {
		typeName: "sirens_echo.telemetry.unclassified",
		message:  "An unclassified runtime failure occurred.",
		stage:    "telemetry",
		outcome:  "unclassified",
		fault:    faultService,
	},
	exceptionTurnFailed: {
		typeName: "sirens_echo.turn.failed",
		message:  "Turn processing failed.",
		stage:    "turn",
		outcome:  "failed",
		fault:    faultService,
	},
	exceptionHistoryFailed: {
		typeName: "sirens_echo.history.read_failed",
		message:  "Message history retrieval failed.",
		stage:    "history",
		outcome:  "read_failed",
		fault:    faultService,
	},
	exceptionResponseValidationFailed: {
		typeName: "sirens_echo.response.validation_failed",
		message:  "Response validation failed.",
		stage:    "validation",
		outcome:  "failed",
		fault:    faultService,
	},
	exceptionDiscordReplyFailed: {
		typeName: "sirens_echo.discord.reply_failed",
		message:  "Discord reply delivery failed.",
		stage:    "reply",
		outcome:  "discord_failed",
		fault:    faultService,
	},
	exceptionReplyFailed: {
		typeName: "sirens_echo.reply.failed",
		message:  "Reply delivery failed.",
		stage:    "reply",
		outcome:  "failed",
		fault:    faultService,
	},
	exceptionMCPToolsListFailed: {
		typeName: "sirens_echo.mcp.tools_list_failed",
		message:  "MCP tool discovery failed.",
		stage:    "mcp",
		outcome:  "tools_list_failed",
		fault:    faultService,
	},
	exceptionMCPServerUnavailable: {
		typeName: "sirens_echo.mcp.server_unavailable",
		message:  "An MCP server did not answer and contributed no tools.",
		stage:    "mcp",
		outcome:  "server_unavailable",
		fault:    faultService,
	},
	exceptionMCPSessionCloseFailed: {
		typeName: "sirens_echo.mcp.session_close_failed",
		message:  "MCP session cleanup failed.",
		stage:    "mcp",
		outcome:  "session_close_failed",
		fault:    faultService,
	},
	exceptionMCPToolCallFailed: {
		typeName: "sirens_echo.mcp.tool_call_failed",
		message:  "MCP tool call failed.",
		stage:    "mcp",
		outcome:  "tool_call_failed",
		fault:    faultService,
	},
	exceptionModelRequestMarshalFailed: {
		typeName: "sirens_echo.model.request_marshal_failed",
		message:  "Agent Proxy request encoding failed.",
		stage:    "model",
		outcome:  "request_marshal_failed",
		fault:    faultService,
	},
	exceptionModelRequestBuildFailed: {
		typeName: "sirens_echo.model.request_build_failed",
		message:  "Agent Proxy request construction failed.",
		stage:    "model",
		outcome:  "request_build_failed",
		fault:    faultService,
	},
	exceptionModelTransportFailed: {
		typeName: "sirens_echo.model.transport_failed",
		message:  "Agent Proxy transport failed.",
		stage:    "model",
		outcome:  "transport_failed",
		fault:    faultService,
	},
	exceptionModelResponseReadFailed: {
		typeName: "sirens_echo.model.response_read_failed",
		message:  "Agent Proxy response read failed.",
		stage:    "model",
		outcome:  "response_read_failed",
		fault:    faultService,
	},
	exceptionModelResponseTooLarge: {
		typeName: "sirens_echo.model.response_too_large",
		message:  "Agent Proxy response exceeded the size limit.",
		stage:    "model",
		outcome:  "response_too_large",
		fault:    faultService,
	},
	exceptionModelResponseHTTPError: {
		typeName: "sirens_echo.model.response_http_error",
		message:  "Agent Proxy returned an unsuccessful HTTP status.",
		stage:    "model",
		outcome:  "response_http_error",
		fault:    faultService,
	},
	exceptionModelResponseDecodeFailed: {
		typeName: "sirens_echo.model.response_decode_failed",
		message:  "Agent Proxy response decoding failed.",
		stage:    "model",
		outcome:  "response_decode_failed",
		fault:    faultService,
	},
	exceptionModelResponseMissingChoice: {
		typeName: "sirens_echo.model.response_missing_choice",
		message:  "Agent Proxy response contained no choice.",
		stage:    "model",
		outcome:  "response_missing_choice",
		fault:    faultService,
	},
	// Silence is not the same failure as a slow answer, and the whole point of
	// the idle timeout is being able to say which. See sirens-echo#171.
	exceptionModelSilent: {
		typeName: "sirens_echo.model.silent",
		message:  "The model backend sent nothing within the idle timeout.",
		stage:    "model",
		outcome:  "silent",
		fault:    faultService,
	},
	exceptionHTTPTurnMethodNotAllowed: {
		typeName: "sirens_echo.http.turn_method_not_allowed",
		message:  "The HTTP turn method is not allowed.",
		stage:    "http",
		outcome:  "method_not_allowed",
		fault:    faultCaller,
	},
	exceptionHTTPTurnInvalidJSON: {
		typeName: "sirens_echo.http.turn_invalid_json",
		message:  "The HTTP turn body is not valid JSON.",
		stage:    "http",
		outcome:  "invalid_json",
		fault:    faultCaller,
	},
	// Separate from input_too_long, which is the post-decode field caps. One
	// bucket for two limits leaves sirens-echo#159 unable to tell them apart.
	exceptionHTTPTurnBodyTooLarge: {
		typeName: "sirens_echo.http.turn_body_too_large",
		message:  "The HTTP turn body exceeded the request size limit.",
		stage:    "http",
		outcome:  "body_too_large",
		fault:    faultCaller,
	},
	exceptionHTTPTurnUnknownField: {
		typeName: "sirens_echo.http.turn_unknown_field",
		message:  "The HTTP turn body carries a field this contract does not define.",
		stage:    "http",
		outcome:  "unknown_field",
		fault:    faultCaller,
	},
	exceptionHTTPTurnContentRequired: {
		typeName: "sirens_echo.http.turn_content_required",
		message:  "The HTTP turn content is missing.",
		stage:    "http",
		outcome:  "content_required",
		fault:    faultCaller,
	},
	exceptionHTTPTurnInputTooLong: {
		typeName: "sirens_echo.http.turn_input_too_long",
		message:  "The HTTP turn input exceeded the size limit.",
		stage:    "http",
		outcome:  "input_too_long",
		fault:    faultCaller,
	},
	exceptionHTTPTurnHistoryTooLong: {
		typeName: "sirens_echo.http.turn_history_too_long",
		message:  "The HTTP turn history exceeded the context limit.",
		stage:    "http",
		outcome:  "history_too_long",
		fault:    faultCaller,
	},
	exceptionHTTPTurnPromptFailed: {
		typeName: "sirens_echo.http.turn_prompt_failed",
		message:  "The selected MCP prompt could not be resolved.",
		stage:    "http",
		outcome:  "prompt_failed",
		fault:    faultService,
	},
	exceptionHTTPTurnRateLimited: {
		typeName: "sirens_echo.http.turn_rate_limited",
		message:  "The HTTP turn exceeded the admission policy.",
		stage:    "http",
		outcome:  "rate_limited",
		fault:    faultService,
	},
	// The jobs stage. Its own stage rather than http, because a job outlives
	// the request that submitted it. See docs/sirens-echo-jobs-lifecycle.md.
	exceptionJobRequestInvalid: {
		typeName: "sirens_echo.jobs.request_invalid",
		message:  "The job request body is not valid JSON.",
		stage:    "jobs",
		outcome:  "request_invalid",
		fault:    faultCaller,
	},
	exceptionJobBodyTooLarge: {
		typeName: "sirens_echo.jobs.body_too_large",
		message:  "The job request body exceeded the request size limit.",
		stage:    "jobs",
		outcome:  "body_too_large",
		fault:    faultCaller,
	},
	exceptionJobRejected: {
		typeName: "sirens_echo.jobs.rejected",
		message:  "The job submission was refused.",
		stage:    "jobs",
		outcome:  "rejected",
		fault:    faultCaller,
	},
	// Separate from rejected, because a refusal a retry can never satisfy must
	// not read as a request the caller could correct. See sirens-echo#825.
	exceptionJobNotPermitted: {
		typeName: "sirens_echo.jobs.not_permitted",
		message:  "The principal is not granted this job kind.",
		stage:    "jobs",
		outcome:  "not_permitted",
		fault:    faultCaller,
	},
	exceptionJobNotFound: {
		typeName: "sirens_echo.jobs.not_found",
		message:  "The job does not exist, or belongs to another principal.",
		stage:    "jobs",
		outcome:  "not_found",
		fault:    faultCaller,
	},
	// The only one of the five that is not the caller's fault. A full queue is
	// this service declining work it could not schedule.
	exceptionJobQueueFull: {
		typeName: "sirens_echo.jobs.queue_full",
		message:  "The job queue is full.",
		stage:    "jobs",
		outcome:  "queue_full",
		fault:    faultService,
	},
	// A declared verb exiting non-zero is this deployment's job failing, not a
	// member asking for something wrong.
	exceptionCommandFailed: {
		typeName: "sirens_echo.command.failed",
		message:  "A workspace command exited non-zero or did not run.",
		stage:    "command",
		outcome:  "failed",
		fault:    faultService,
	},
	// The member uploaded a file and this service could not fetch it, which is
	// the CDN or this egress path rather than anything they did.
	exceptionAttachmentFetchFailed: {
		typeName: "sirens_echo.attachment.fetch_failed",
		message:  "An attachment could not be fetched.",
		stage:    "attachment",
		outcome:  "fetch_failed",
		fault:    faultService,
	},
	// Both gate failures are the service's, including the unknown class: the
	// model answering off its own closed list is not something a member did.
	exceptionContentGateModelFailed: {
		typeName: "sirens_echo.content_gate.model_failed",
		message:  "The content classifier call failed.",
		stage:    "content_gate",
		outcome:  "model_failed",
		fault:    faultService,
	},
	exceptionContentGateUnknownClass: {
		typeName: "sirens_echo.content_gate.unknown_class",
		message:  "The content classifier answered outside its closed class list.",
		stage:    "content_gate",
		outcome:  "unknown_class",
		fault:    faultService,
	},
}

func exceptionFor(code exceptionCode) exceptionSpec {
	if code >= exceptionCodeCount {
		return exceptionCatalog[exceptionUnclassified]
	}
	spec := exceptionCatalog[code]
	if spec.typeName == "" || spec.message == "" || spec.stage == "" || spec.outcome == "" {
		return exceptionCatalog[exceptionUnclassified]
	}
	return spec
}

// MarkSpanError records one cataloged exception and marks its span as failed.
// The typed catalog prevents dynamic runtime data from entering exception fields.
func (t *Telemetry) MarkSpanError(span trace.Span, code exceptionCode) {
	spec := exceptionFor(code)
	attributes := []attribute.KeyValue{
		attribute.String("exception.type", spec.typeName),
		attribute.String("exception.message", spec.message),
		attribute.String("error.stage", spec.stage),
		attribute.String("error.outcome", spec.outcome),
		attribute.String("error.fault", spec.fault),
	}
	span.AddEvent("exception", trace.WithAttributes(attributes...))
	span.SetAttributes(
		attribute.String("error.type", spec.typeName),
		attribute.String("error.stage", spec.stage),
		attribute.String("error.outcome", spec.outcome),
		attribute.String("error.fault", spec.fault),
	)
	span.SetStatus(codes.Error, spec.message)
}
