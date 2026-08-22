package community

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// interruptibleTurn is the Discord shape in the part that matters here: a
// summon that can still be named after the process answering it is gone.
type interruptibleTurn struct {
	*httpTurn
	channelID string
}

func (t interruptibleTurn) InterruptRecord() TurnRecord {
	return TurnRecord{
		MessageID: t.requestID,
		ChannelID: t.channelID,
		Author:    "member-id",
	}
}

func recordingAgent(t *testing.T, log TurnLog) *Agent {
	t.Helper()
	agent := &Agent{
		cfg:          Config{Definition: Definition{MaxContextMessages: 12}},
		completions:  answeringClient{reply: "Echo is ready."},
		systemPrompt: "neutral model policy and local knowledge",
		telemetry:    telemetryOrNoop(nil),
		turns:        log,
	}
	agent.ensureRuntimeDefaults()
	return agent
}

func summon(id string) interruptibleTurn {
	return interruptibleTurn{
		httpTurn: &httpTurn{
			requestID: id,
			current:   TranscriptEntry{Author: "member", Content: "are you ready?"},
		},
		channelID: "channel-1",
	}
}

// Without this the clearing tests below pass on a harness that never writes a
// record at all. See docs/sirens-echo-execution.md.
func TestATurnMarksItselfInProgressBeforeItCallsTheModel(t *testing.T) {
	t.Parallel()
	log := NewMemoryTurnLog()
	client := blockingClient{arrived: make(chan struct{}, 1), release: make(chan struct{})}
	agent := recordingAgent(t, log)
	agent.completions = client

	go func() { _ = agent.runAdmitted(context.Background(), summon("running")) }()
	awaitArrivals(t, client, 1)

	swept, err := log.Sweep("another-run")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	close(client.release)
	if len(swept) != 1 || swept[0].MessageID != "running" {
		t.Fatalf("swept %+v, want the running turn marked in progress", swept)
	}
	if swept[0].Process != agent.process {
		t.Errorf("record process = %q, want the run that wrote it", swept[0].Process)
	}
	if swept[0].StartedAt.IsZero() {
		t.Error("the record carries no start time, so a boot cannot say how old it is")
	}
}

// A record left by a turn that finished would be reported as interrupted at the
// next boot, which is a false alarm rather than a missing one.
func TestATurnThatFinishesLeavesNoRecord(t *testing.T) {
	t.Parallel()
	log := NewMemoryTurnLog()
	agent := recordingAgent(t, log)

	if err := agent.runAdmitted(context.Background(), summon("finished")); err != nil {
		t.Fatalf("runAdmitted: %v", err)
	}
	swept, err := log.Sweep("another-run")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 0 {
		t.Errorf("a completed turn left %d records behind", len(swept))
	}
}

// The failure path reaches the end of the turn as much as the success path
// does, so it owes the same clearing.
func TestAFailedTurnAlsoLeavesNoRecord(t *testing.T) {
	t.Parallel()
	log := NewMemoryTurnLog()
	agent := recordingAgent(t, log)
	agent.completions = answeringClient{reply: ""}

	_ = agent.runAdmitted(context.Background(), summon("failed"))

	swept, err := log.Sweep("another-run")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 0 {
		t.Errorf("a failed turn left %d records behind", len(swept))
	}
}

// The turn the roll took. Nothing runs at the death, so the record is what the
// last run wrote at the start and never cleared. See sirens-echo#989.
func TestAnInterruptedTurnIsExactlyOneRecordNamingItsMessageAndChannel(t *testing.T) {
	t.Parallel()
	log := NewMemoryTurnLog()
	dying := recordingAgent(t, log)
	if err := dying.turns.Begin(TurnRecord{
		MessageID: "summon-9", ChannelID: "channel-1", Author: "member-id",
		Process: dying.process, StartedAt: time.Unix(1700000000, 0).UTC(),
	}); err != nil {
		t.Fatalf("Begin: %v", err)
	}

	booting := recordingAgent(t, log)
	swept, err := log.Sweep(booting.process)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 1 {
		t.Fatalf("swept %d records, want exactly the interrupted turn", len(swept))
	}
	if swept[0].MessageID != "summon-9" || swept[0].ChannelID != "channel-1" {
		t.Errorf("record = %+v, want the message and channel named", swept[0])
	}
}

// A sweep that took this run's own turns would report a live turn as lost and
// clear the record that would have reported it if it really were.
func TestASweepLeavesThisRunsOwnTurnsAlone(t *testing.T) {
	t.Parallel()
	log := NewMemoryTurnLog()
	agent := recordingAgent(t, log)
	if err := log.Begin(TurnRecord{MessageID: "live", Process: agent.process}); err != nil {
		t.Fatalf("Begin: %v", err)
	}

	swept, err := log.Sweep(agent.process)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 0 {
		t.Errorf("the sweep took %d of this run's own live turns", len(swept))
	}
}

// The boot report is once, because a second boot finding the same record would
// tell the room again about a summon it has already been told about.
func TestTheBootReportClearsWhatItReported(t *testing.T) {
	t.Parallel()
	log := NewMemoryTurnLog()
	if err := log.Begin(TurnRecord{
		MessageID: "summon-9", ChannelID: "channel-1", Process: "an-older-run",
	}); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	agent := recordingAgent(t, log)

	agent.reportInterruptedTurns(context.Background())

	again, err := log.Sweep("a-third-run")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("%d records survived the report that consumed them", len(again))
	}
}

// The file log is the half that makes SIGKILL and SIGTERM produce the same
// record: it was written at the start, so no shutdown path has to run.
func TestAFileTurnLogOutlivesTheRunThatWroteIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writing, err := OpenFileTurnLog(dir)
	if err != nil {
		t.Fatalf("OpenFileTurnLog: %v", err)
	}
	if err := writing.Begin(TurnRecord{
		MessageID: "summon-9", ChannelID: "channel-1", Process: "the-killed-run",
		StartedAt: time.Unix(1700000000, 0).UTC(),
	}); err != nil {
		t.Fatalf("Begin: %v", err)
	}

	// No Finish and no shutdown hook, which is the whole point.
	booted, err := OpenFileTurnLog(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	swept, err := booted.Sweep("the-booting-run")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 1 || swept[0].MessageID != "summon-9" {
		t.Fatalf("swept %+v, want the record the killed run left", swept)
	}
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("%d files survived the sweep that reported them", len(left))
	}
}

// A finished turn on the file log clears the file too, or the directory grows
// one record per turn forever and every boot reports the lot.
func TestTheFileLogClearsTheFileWhenTheTurnFinishes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log, err := OpenFileTurnLog(dir)
	if err != nil {
		t.Fatalf("OpenFileTurnLog: %v", err)
	}
	if err := log.Begin(TurnRecord{MessageID: "summon-9", Process: "run"}); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := log.Finish("summon-9"); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("%d files survived a finished turn", len(left))
	}
}

// The selection is the deployment contract: the turn log follows the store
// rather than taking a name of its own.
func TestTheTurnLogFollowsTheJobStoreSelection(t *testing.T) {
	t.Parallel()
	memory, err := openTurnLog(Config{}, NewMemoryJobStore(nil))
	if err != nil {
		t.Fatalf("unconfigured: %v", err)
	}
	if _, ok := memory.(*MemoryTurnLog); !ok {
		t.Errorf("unconfigured log = %T, want the memory log", memory)
	}

	dir := t.TempDir()
	store, err := OpenFileJobStore(dir, nil)
	if err != nil {
		t.Fatalf("OpenFileJobStore: %v", err)
	}
	file, err := openTurnLog(Config{JobStoreDir: dir}, store)
	if err != nil {
		t.Fatalf("directory: %v", err)
	}
	if _, ok := file.(*FileTurnLog); !ok {
		t.Errorf("directory log = %T, want the file log", file)
	}
}

// Two processes must never share a run identity, or every record reads as the
// reader's own live turn and nothing is ever reported.
func TestEachRunGetsItsOwnIdentity(t *testing.T) {
	t.Parallel()
	first, second := newProcessID(), newProcessID()
	if first == second {
		t.Errorf("two runs share the identity %q", first)
	}
	if strings.TrimSpace(first) == "" {
		t.Error("a run identity is empty, so every record reads as this run's own")
	}
}

// postgresTestTurnLog opens the log against a real database, or skips. The
// alternative is no SQL coverage at all. See docs/sirens-echo-jobs.md.
func postgresTestTurnLog(t *testing.T) *PostgresTurnLog {
	t.Helper()
	store := postgresTestStore(t)
	log, err := OpenPostgresTurnLog(context.Background(), store.pool)
	if err != nil {
		t.Fatalf("OpenPostgresTurnLog: %v", err)
	}
	t.Cleanup(func() {
		if _, err := store.pool.Exec(context.Background(), `DROP TABLE IF EXISTS turns`); err != nil {
			t.Errorf("drop scratch table: %v", err)
		}
	})
	return log
}

// The database is a separate Deployment, so it is reachable both from the pod
// that dies and from the pod that boots. That is the whole reason it is here.
func TestThePostgresTurnLogReportsWhatAnotherRunLeft(t *testing.T) {
	log := postgresTestTurnLog(t)
	started := time.Unix(1700000000, 0).UTC()
	if err := log.Begin(TurnRecord{
		MessageID: "summon-9", ChannelID: "channel-1", Author: "member-id",
		Process: "the-killed-run", StartedAt: started,
	}); err != nil {
		t.Fatalf("Begin: %v", err)
	}

	swept, err := log.Sweep("the-booting-run")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 1 {
		t.Fatalf("swept %d records, want the one the killed run left", len(swept))
	}
	if swept[0].MessageID != "summon-9" || swept[0].ChannelID != "channel-1" {
		t.Errorf("record = %+v, want the message and channel named", swept[0])
	}
	if !swept[0].StartedAt.Equal(started) {
		t.Errorf("started at %s, want %s", swept[0].StartedAt, started)
	}
}

// A sweep that left the row behind would report the same summon at every boot
// from then on, and two pods booting together would both report it.
func TestThePostgresTurnLogTakesEachRecordOnce(t *testing.T) {
	log := postgresTestTurnLog(t)
	if err := log.Begin(TurnRecord{
		MessageID: "summon-9", Process: "the-killed-run",
		StartedAt: time.Unix(1700000000, 0).UTC(),
	}); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if _, err := log.Sweep("the-booting-run"); err != nil {
		t.Fatalf("first Sweep: %v", err)
	}

	again, err := log.Sweep("a-third-run")
	if err != nil {
		t.Fatalf("second Sweep: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("%d records survived the sweep that took them", len(again))
	}
}

// A finished turn clears its row, and a live turn of the sweeping run keeps
// its own. Both in one test because they are one statement's two halves.
func TestThePostgresTurnLogClearsFinishedAndKeepsLive(t *testing.T) {
	log := postgresTestTurnLog(t)
	for _, record := range []TurnRecord{
		{MessageID: "finished", Process: "this-run", StartedAt: time.Unix(1700000000, 0).UTC()},
		{MessageID: "live", Process: "this-run", StartedAt: time.Unix(1700000001, 0).UTC()},
	} {
		if err := log.Begin(record); err != nil {
			t.Fatalf("Begin %s: %v", record.MessageID, err)
		}
	}
	if err := log.Finish("finished"); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	swept, err := log.Sweep("this-run")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 0 {
		t.Errorf("the sweep took %d of this run's own records", len(swept))
	}
	stranded, err := log.Sweep("a-later-run")
	if err != nil {
		t.Fatalf("later Sweep: %v", err)
	}
	if len(stranded) != 1 || stranded[0].MessageID != "live" {
		t.Errorf("stranded = %+v, want only the turn that never finished", stranded)
	}
}
