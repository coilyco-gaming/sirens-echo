package community

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// A scratchpad partition is a session, not a requester. See
// docs/sirens-echo-scratchpad.md for the layout and both timers.

const (
	// threadSessionPrefix and directSessionPrefix name which timer collects a
	// session, so the sweeper needs no state beside the directory itself.
	threadSessionPrefix = "t-"
	directSessionPrefix = "c-"
)

// SessionKind is which retention rule a session takes. A thread is a bounded
// conversation people return to; a channel pairing has no natural end.
type SessionKind string

const (
	// SessionThread is a Discord thread, collected after a long quiet period.
	SessionThread SessionKind = "thread"
	// SessionDirect is the channel-and-user fallback, collected on idle.
	SessionDirect SessionKind = "direct"
)

// SessionID is the workspace a turn shares. Two members in one thread resolve
// to the same value, which is what makes the workspace shared.
type SessionID struct {
	Kind SessionKind
	// Key is the raw identity, hashed before it reaches a path so no member
	// identifier lands where the model reads it.
	Key string
}

// Valid reports whether this identifies a workspace. An empty key partitions
// nothing, so absence is denial rather than a shared bucket.
func (s SessionID) Valid() bool { return strings.TrimSpace(s.Key) != "" }

// Directory is the flat, opaque name this session takes on disk. The prefix
// carries the kind so a sweeper reads the rule off the entry.
func (s SessionID) Directory() string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(s.Key)))
	name := hex.EncodeToString(sum[:16])
	if s.Kind == SessionThread {
		return threadSessionPrefix + name
	}
	return directSessionPrefix + name
}

// ThreadSession is the workspace of one Discord thread, shared by everyone in
// it. See docs/sirens-echo-scratchpad.md.
func ThreadSession(threadID string) SessionID {
	return SessionID{Kind: SessionThread, Key: "thread:" + strings.TrimSpace(threadID)}
}

// DirectSession is the channel-and-user pairing used outside a thread, where
// no bounded conversation exists to share.
func DirectSession(channelID, requesterID string) SessionID {
	return SessionID{
		Kind: SessionDirect,
		Key:  "channel:" + strings.TrimSpace(channelID) + "\x00" + strings.TrimSpace(requesterID),
	}
}

// sessionKindOf reads the rule back off a directory name written by Directory.
func sessionKindOf(directory string) (SessionKind, bool) {
	switch {
	case strings.HasPrefix(directory, threadSessionPrefix):
		return SessionThread, true
	case strings.HasPrefix(directory, directSessionPrefix):
		return SessionDirect, true
	default:
		return "", false
	}
}

type sessionKey struct{}

// WithSession marks a context as belonging to one workspace.
func WithSession(ctx context.Context, id SessionID) context.Context {
	if !id.Valid() {
		return ctx
	}
	return context.WithValue(ctx, sessionKey{}, id)
}

// SessionFrom reports the workspace a context carries.
func SessionFrom(ctx context.Context) SessionID {
	value, _ := ctx.Value(sessionKey{}).(SessionID)
	return value
}

// sessionOwner is an optional turn capability, declared by a transport that
// knows which conversation a turn belongs to.
type sessionOwner interface {
	SessionID() SessionID
}

// sessionOf resolves the workspace for a turn. A transport with no notion of a
// conversation still gets a stable private one rather than sharing.
func sessionOf(turn turnIO) SessionID {
	if owner, ok := turn.(sessionOwner); ok {
		if id := owner.SessionID(); id.Valid() {
			return id
		}
	}
	return DirectSession(turn.Transport(), turn.Requester())
}

// sessionAge is how long since the newest file changed. See
// docs/sirens-echo-scratchpad.md.
func sessionAge(dir string, now time.Time) (time.Duration, error) {
	newest := time.Time{}
	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Files only. Removing one moves its directory's timestamp, so counting
		// directories would let eviction keep a dead session alive.
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if !newest.IsZero() {
		return now.Sub(newest), nil
	}
	// No files at all, so the session's own directory is the only evidence and
	// an empty workspace still expires.
	info, statErr := os.Stat(dir)
	if statErr != nil {
		return 0, statErr
	}
	return now.Sub(info.ModTime()), nil
}

// retentionFor is how long a session of this kind may sit untouched.
func retentionFor(kind SessionKind) time.Duration {
	if kind == SessionThread {
		return threadSessionRetention
	}
	return directSessionRetention
}

// CollectSessions removes every workspace past its retention. It returns the
// directories it removed, so a caller can report what a member lost.
func CollectSessions(root string, now time.Time) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read scratchpad root: %w", err)
	}
	collected := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		kind, known := sessionKindOf(entry.Name())
		// A foreign directory is left alone rather than guessed at.
		if !known {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		idle, ageErr := sessionAge(dir, now)
		if ageErr != nil {
			return collected, ageErr
		}
		if idle < retentionFor(kind) {
			continue
		}
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			return collected, fmt.Errorf("collect session %s: %w", entry.Name(), removeErr)
		}
		collected = append(collected, entry.Name())
	}
	sort.Strings(collected)
	return collected, nil
}

// sweepSessions collects expired workspaces on a timer and returns a stop.
// A deployment with no scratchpad runs nothing rather than an empty loop.
func (a *Agent) sweepSessions(ctx context.Context) func() {
	root := strings.TrimSpace(a.cfg.ScratchDir)
	if root == "" {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(sessionSweepEvery)
		defer ticker.Stop()
		// Once at startup, so a restart does not hold expired files.
		a.collectExpiredSessions(ctx, root)
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				a.collectExpiredSessions(ctx, root)
			}
		}
	}()
	return func() { close(done) }
}

// collectExpiredSessions runs one pass and reports what it removed.
func (a *Agent) collectExpiredSessions(ctx context.Context, root string) {
	collected, err := CollectSessions(root, time.Now())
	if err != nil {
		a.telemetry.Error(ctx, "scratch.session.sweep.failed", slog.String("error", err.Error()))
		return
	}
	for _, session := range collected {
		kind, _ := sessionKindOf(session)
		a.telemetry.Info(
			ctx,
			"scratch.session.collected",
			slog.String("session", session),
			slog.String("session.kind", string(kind)),
			slog.String("reason", "retention"),
		)
	}
}
