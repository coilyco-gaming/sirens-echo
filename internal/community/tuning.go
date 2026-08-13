package community

import "time"

// Every tuning number this package has, in one file. See
// docs/sirens-echo-tuning.md for why they are here and how to change one.

// Model calls: rounds, repairs, and the completion budget ladder
const (
	maxToolRounds      = 6
	maxResponseRepairs = 1
	// maxToolResultBytes bounds one tool result before it re-enters the
	// prompt. Four parallel Eco calls inflated a 6k prompt past 47k.
	maxToolResultBytes = 8 * 1024
	// Completion budget, escalated rather than fixed.
	// See docs/sirens-echo-budget.md.
	baseCompletionTokens = 1800
	maxCompletionTokens  = 3600
	completionBudgetStep = 2
	// budgetRaisesAllowed bounds the escalation so a pathological turn cannot
	// loop. One real rung remains: 1800 to 3600, then exhausted.
	budgetRaisesAllowed = 1
)

// MCP: refresh, timeouts, backoff, and grounding size
const (
	// defaultRosterRefresh bounds staleness for a transport that cannot push
	// tools/list_changed. See docs/sirens-echo-mcp-roster.md.
	defaultRosterRefresh = time.Hour
	mcpConnectTimeout    = 10 * time.Second
	mcpListTimeout       = 15 * time.Second
	mcpBackoffMin        = 5 * time.Second
	mcpBackoffMax        = 2 * time.Minute
	// defaultCallTimeout keeps one tool call well inside the turn budget, so a
	// server that never answers cannot spend the whole turn.
	defaultCallTimeout = 45 * time.Second
	// Grounding bounds. Reference material must not crowd out the turn it is
	// meant to support.
	maxGroundingBytes     = 8 * 1024
	maxGroundingDocuments = 8
)

// Turn progress cadence. Only the wait is written down
const (
	// turnProgressAfter is how long a turn runs before it starts reporting. A
	// reply that beats this never posts anything.
	turnProgressAfter = 3 * time.Second
	// turnProgressEvery is the grid every later message releases on, so an edit,
	// a reply, and a failure notice all land on the same beat.
	turnProgressEvery = turnProgressAfter * 2
	// turnLongReplyAfter is when a turn has taken long enough that its reply
	// wants somewhere of its own. Derived, so there is one number to move.
	turnLongReplyAfter = turnProgressAfter + turnProgressEvery*2
)

// Job progress
const ()

// Job execution
const (
	// defaultJobQueueDepth bounds accepted-but-unstarted work. A full queue
	// refuses rather than growing without limit.
	defaultJobQueueDepth = 64
	// defaultJobTimeout bounds one execution. Jobs are long, not unbounded.
	defaultJobTimeout = 30 * time.Minute
)

// Turn timeouts
const (
	defaultRequestTimeout = 3 * time.Minute
	// defaultQueueTimeout bounds the wait for the execution slot. A longer
	// wait answers a conversation that has already moved on.
	defaultQueueTimeout = 30 * time.Second
)

// Workspace commands
const (
	// defaultCommandTimeout bounds one command inside a job's own budget.
	defaultCommandTimeout = 10 * time.Minute
)

// Readiness probe
const (
	defaultReadinessTimeout = 5 * time.Second
)

// Agent-to-agent exchange bound
const (
	// maxAgentExchange bounds consecutive agent-to-agent turns in one channel,
	// because two agents each answering the other is a runaway.
	maxAgentExchange = 4
	// agentExchangeWindow is how long a run of agent turns stays counted. A
	// quiet channel forgets, so an exchange later is a fresh one.
	agentExchangeWindow = 10 * time.Minute
)

// Attachment ingest
const (
	// maxAttachmentBytes stays under the scratchpad's per-file limit, so an
	// oversized upload refuses here with a reason rather than there.
	maxAttachmentBytes = 128 * 1024
	// attachmentFetchTimeout bounds one download inside a turn that already
	// owes the member an answer.
	attachmentFetchTimeout = 10 * time.Second
)

// Scratch space
const (
	// maxScratchFileBytes bounds one file. A Discord message can ask for an
	// unbounded write and the volume is shared with the pod.
	maxScratchFileBytes = 256 * 1024
	// maxScratchEntries bounds a listing, keeping a result inside the turn
	// budget rather than returning a directory of unknown size.
	maxScratchEntries = 200
	// maxScratchMatches bounds a search result for the same reason.
	maxScratchMatches = 100
	// maxScratchDepth bounds nesting so a walk stays cheap.
	maxScratchDepth = 8
)

// Slash command shape, bounded by Discord
const (
	// Discord's limits. A breach fails the whole registration, so one
	// server's prompt would otherwise cost every command.
	maxCommandNameRunes        = 32
	maxCommandDescriptionRunes = 100
	// maxCommandOptions is Discord's ceiling on options per command.
	maxCommandOptions = 25
)

// Block responses
const (
	// maxBlockReasonWords bounds the reason. Every volunteered justification is
	// a handle to pull, and this reply only ever appears at a boundary.
	maxBlockReasonWords = 20
)
