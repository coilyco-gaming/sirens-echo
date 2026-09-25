package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The durable hand-off between the gateway sessions that receive Discord events
// and the worker that answers them. See docs/sirens-echo-jobs.md.

// Discord event kinds a gateway session offers to the queue.
const (
	discordEventMessage     = "message"
	discordEventEdit        = "edit"
	discordEventInteraction = "interaction"
)

// DiscordEvent is one inbound gateway event, kept whole so the worker admits it
// exactly as a connected session would have.
type DiscordEvent struct {
	// Key is the idempotency key. Every session that sees the event derives the
	// same one, so two intakes produce one row.
	Key        string
	Kind       string
	Payload    json.RawMessage
	ReceivedAt time.Time
}

// DiscordEventQueue is shared by every intake and the worker. Unlike JobStore it
// is written by several processes at once, which is what it exists for.
type DiscordEventQueue interface {
	// Offer records an event once. The bool reports whether this call inserted it.
	Offer(ctx context.Context, event DiscordEvent) (bool, error)
	// Claim takes up to limit unclaimed events, oldest first, for owner. A
	// claimed event is never handed out again, so delivery is at most once.
	Claim(ctx context.Context, owner string, limit int) ([]DiscordEvent, error)
	// Sweep deletes events received before cutoff, claimed or not.
	Sweep(ctx context.Context, cutoff time.Time) (int, error)
}

// discordEventQueueSchema is applied at open. A claimed row stays until swept, so
// a late duplicate from a second intake collides rather than being answered.
const discordEventQueueSchema = `
CREATE TABLE IF NOT EXISTS discord_events (
    key         TEXT PRIMARY KEY,
    kind        TEXT NOT NULL,
    payload     JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    claimed_at  TIMESTAMPTZ,
    claimed_by  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS discord_events_unclaimed
    ON discord_events (received_at, key) WHERE claimed_at IS NULL;
`

// PostgresDiscordEventQueue is the queue every intake replica and the worker
// reach, on the database the job store already uses.
type PostgresDiscordEventQueue struct {
	pool  *pgxpool.Pool
	owned bool
}

// OpenPostgresDiscordEventQueue dials its own pool, for the intake process,
// which has no job store to borrow one from.
func OpenPostgresDiscordEventQueue(ctx context.Context, dsn string) (*PostgresDiscordEventQueue, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("the Discord event queue needs SIRENS_ECHO_JOB_STORE_DSN")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open Discord event queue: %w", redactDSN(err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("reach Discord event queue: %w", redactDSN(err))
	}
	queue, err := newPostgresDiscordEventQueue(ctx, pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	queue.owned = true
	return queue, nil
}

// newPostgresDiscordEventQueue ensures the schema on a pool the caller owns.
func newPostgresDiscordEventQueue(ctx context.Context, pool *pgxpool.Pool) (*PostgresDiscordEventQueue, error) {
	if pool == nil {
		return nil, errors.New("the Discord event queue needs a database pool")
	}
	if _, err := pool.Exec(ctx, discordEventQueueSchema); err != nil {
		return nil, fmt.Errorf("create Discord event queue schema: %w", redactDSN(err))
	}
	return &PostgresDiscordEventQueue{pool: pool}, nil
}

// Close releases a pool this queue opened, and leaves a borrowed one alone.
func (q *PostgresDiscordEventQueue) Close() {
	if q.owned && q.pool != nil {
		q.pool.Close()
	}
}

// Ping reports whether the database answers, which is half of intake readiness.
func (q *PostgresDiscordEventQueue) Ping(ctx context.Context) error {
	return redactDSN(q.pool.Ping(ctx))
}

func (q *PostgresDiscordEventQueue) Offer(ctx context.Context, event DiscordEvent) (bool, error) {
	if err := validateDiscordEvent(event); err != nil {
		return false, err
	}
	tag, err := q.pool.Exec(ctx, `
INSERT INTO discord_events (key, kind, payload, received_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (key) DO NOTHING`,
		event.Key, event.Kind, []byte(event.Payload), event.ReceivedAt)
	if err != nil {
		return false, fmt.Errorf("offer Discord event: %w", redactDSN(err))
	}
	return tag.RowsAffected() == 1, nil
}

func (q *PostgresDiscordEventQueue) Claim(ctx context.Context, owner string, limit int) ([]DiscordEvent, error) {
	if limit <= 0 {
		return nil, nil
	}
	// SKIP LOCKED makes a second worker take different rows rather than wait, so
	// scaling the worker cannot answer one event twice.
	rows, err := q.pool.Query(ctx, `
UPDATE discord_events SET claimed_at = now(), claimed_by = $1
WHERE key IN (
    SELECT key FROM discord_events
    WHERE claimed_at IS NULL
    ORDER BY received_at, key
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
RETURNING key, kind, payload, received_at`, owner, limit)
	if err != nil {
		return nil, fmt.Errorf("claim Discord events: %w", redactDSN(err))
	}
	defer rows.Close()
	claimed := make([]DiscordEvent, 0, limit)
	for rows.Next() {
		var event DiscordEvent
		var payload []byte
		if err := rows.Scan(&event.Key, &event.Kind, &payload, &event.ReceivedAt); err != nil {
			return nil, fmt.Errorf("read claimed Discord event: %w", redactDSN(err))
		}
		event.Payload = payload
		claimed = append(claimed, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim Discord events: %w", redactDSN(err))
	}
	// RETURNING carries no order, and admission order is arrival order.
	sortDiscordEvents(claimed)
	return claimed, nil
}

func (q *PostgresDiscordEventQueue) Sweep(ctx context.Context, cutoff time.Time) (int, error) {
	tag, err := q.pool.Exec(ctx, `DELETE FROM discord_events WHERE received_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("sweep Discord events: %w", redactDSN(err))
	}
	return int(tag.RowsAffected()), nil
}

// MemoryDiscordEventQueue has every behaviour but sharing across processes,
// which makes it right for a test and wrong for a deployment.
type MemoryDiscordEventQueue struct {
	mu      sync.Mutex
	events  map[string]DiscordEvent
	claimed map[string]bool
}

// NewMemoryDiscordEventQueue returns an empty queue.
func NewMemoryDiscordEventQueue() *MemoryDiscordEventQueue {
	return &MemoryDiscordEventQueue{events: map[string]DiscordEvent{}, claimed: map[string]bool{}}
}

func (q *MemoryDiscordEventQueue) Offer(_ context.Context, event DiscordEvent) (bool, error) {
	if err := validateDiscordEvent(event); err != nil {
		return false, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, seen := q.events[event.Key]; seen {
		return false, nil
	}
	q.events[event.Key] = event
	return true, nil
}

func (q *MemoryDiscordEventQueue) Claim(_ context.Context, _ string, limit int) ([]DiscordEvent, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	open := make([]DiscordEvent, 0, len(q.events))
	for key, event := range q.events {
		if !q.claimed[key] {
			open = append(open, event)
		}
	}
	sortDiscordEvents(open)
	if len(open) > limit {
		open = open[:limit]
	}
	for _, event := range open {
		q.claimed[event.Key] = true
	}
	return open, nil
}

func (q *MemoryDiscordEventQueue) Sweep(_ context.Context, cutoff time.Time) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	swept := 0
	for key, event := range q.events {
		if event.ReceivedAt.Before(cutoff) {
			delete(q.events, key)
			delete(q.claimed, key)
			swept++
		}
	}
	return swept, nil
}

func validateDiscordEvent(event DiscordEvent) error {
	switch {
	case strings.TrimSpace(event.Key) == "":
		return errors.New("a Discord event needs an idempotency key")
	case event.Kind != discordEventMessage && event.Kind != discordEventEdit &&
		event.Kind != discordEventInteraction:
		return fmt.Errorf("unknown Discord event kind %q", event.Kind)
	case len(event.Payload) == 0:
		return errors.New("a Discord event needs its payload")
	case event.ReceivedAt.IsZero():
		return errors.New("a Discord event needs the time it was received")
	}
	return nil
}

func sortDiscordEvents(events []DiscordEvent) {
	sort.Slice(events, func(i, j int) bool {
		if !events[i].ReceivedAt.Equal(events[j].ReceivedAt) {
			return events[i].ReceivedAt.Before(events[j].ReceivedAt)
		}
		return events[i].Key < events[j].Key
	})
}
