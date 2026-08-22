package community

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A turn lives in the process, so a roll takes it with no trace at all. The
// record turns that silence into a report. See docs/sirens-echo-execution.md.

// The turn outcomes, a closed set, because every one of them is a metric label.
const (
	turnOutcomeOK    = "ok"
	turnOutcomeError = "error"
	// turnOutcomeInterrupted is recorded at the next boot rather than by the
	// turn, which by then does not exist to record anything.
	turnOutcomeInterrupted = "interrupted"
)

// TurnRecord names one turn that was running when the record was written.
type TurnRecord struct {
	MessageID string    `json:"message_id"`
	ChannelID string    `json:"channel_id"`
	Author    string    `json:"author"`
	StartedAt time.Time `json:"started_at"`
	// Process is the run that wrote it. A record carrying anyone else's run is
	// an interruption rather than a turn still going.
	Process string `json:"process"`
}

// TurnLog holds a record for exactly as long as its turn runs.
type TurnLog interface {
	// Begin records a turn that is about to run.
	Begin(record TurnRecord) error
	// Finish clears the record, on success and on handled failure alike.
	Finish(messageID string) error
	// Sweep takes and clears every record another run left behind, oldest
	// first. It never returns this run's own live turns.
	Sweep(process string) ([]TurnRecord, error)
}

// MemoryTurnLog is every behaviour except the one that matters, so it is right
// for a test and reports nothing after the restart it exists to describe.
type MemoryTurnLog struct {
	mu      sync.Mutex
	records map[string]TurnRecord
}

func NewMemoryTurnLog() *MemoryTurnLog {
	return &MemoryTurnLog{records: make(map[string]TurnRecord)}
}

func (l *MemoryTurnLog) Begin(record TurnRecord) error {
	if strings.TrimSpace(record.MessageID) == "" {
		return errors.New("a turn record needs the message it answers")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records[record.MessageID] = record
	return nil
}

func (l *MemoryTurnLog) Finish(messageID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.records, messageID)
	return nil
}

func (l *MemoryTurnLog) Sweep(process string) ([]TurnRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	swept := make([]TurnRecord, 0, len(l.records))
	for id, record := range l.records {
		if record.Process == process {
			continue
		}
		swept = append(swept, record)
		delete(l.records, id)
	}
	sortRecords(swept)
	return swept, nil
}

// sortRecords orders oldest first, so a boot report reads in the order the
// summons arrived rather than in map order.
func sortRecords(records []TurnRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].StartedAt.Equal(records[j].StartedAt) {
			return records[i].MessageID < records[j].MessageID
		}
		return records[i].StartedAt.Before(records[j].StartedAt)
	})
}

// FileTurnLog persists one record per file, so a mounted volume outlives the
// pod that wrote it.
type FileTurnLog struct {
	*MemoryTurnLog
	dir string
}

// OpenFileTurnLog loads whatever the last run left, which is the half that
// makes a restart reportable rather than merely survivable.
func OpenFileTurnLog(dir string) (*FileTurnLog, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("turn log directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create turn log: %w", err)
	}
	log := &FileTurnLog{MemoryTurnLog: NewMemoryTurnLog(), dir: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read turn log: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read turn record %s: %w", entry.Name(), err)
		}
		var record TurnRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, fmt.Errorf("parse turn record %s: %w", entry.Name(), err)
		}
		if record.MessageID == "" {
			return nil, fmt.Errorf("turn record %s names no message", entry.Name())
		}
		log.records[record.MessageID] = record
	}
	return log, nil
}

func (l *FileTurnLog) Begin(record TurnRecord) error {
	if err := l.MemoryTurnLog.Begin(record); err != nil {
		return err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode turn record: %w", err)
	}
	if err := os.WriteFile(l.path(record.MessageID), raw, 0o644); err != nil {
		// Rolled back, so a failed write cannot leave a record this process
		// believes in and no later boot can find.
		_ = l.MemoryTurnLog.Finish(record.MessageID)
		return fmt.Errorf("write turn record: %w", err)
	}
	return nil
}

func (l *FileTurnLog) Finish(messageID string) error {
	if err := l.MemoryTurnLog.Finish(messageID); err != nil {
		return err
	}
	if err := os.Remove(l.path(messageID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear turn record: %w", err)
	}
	return nil
}

func (l *FileTurnLog) Sweep(process string) ([]TurnRecord, error) {
	swept, err := l.MemoryTurnLog.Sweep(process)
	if err != nil {
		return nil, err
	}
	for _, record := range swept {
		if err := os.Remove(l.path(record.MessageID)); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("clear turn record: %w", err)
		}
	}
	return swept, nil
}

// path keys the file on the message id, which Discord guarantees unique and
// which carries no member text.
func (l *FileTurnLog) path(messageID string) string {
	return filepath.Join(l.dir, messageID+".json")
}

// turnLogSchema is applied at open, the same one-table migration story the job
// store has.
const turnLogSchema = `
CREATE TABLE IF NOT EXISTS turns (
    message_id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL DEFAULT '',
    author     TEXT NOT NULL DEFAULT '',
    process    TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS turns_process ON turns (process, started_at);
`

// PostgresTurnLog writes to the database the job store already reaches, which
// is a separate Deployment and so survives the roll that took the turn.
type PostgresTurnLog struct {
	pool *pgxpool.Pool
}

// OpenPostgresTurnLog ensures the schema on a pool the caller owns.
func OpenPostgresTurnLog(ctx context.Context, pool *pgxpool.Pool) (*PostgresTurnLog, error) {
	if pool == nil {
		return nil, errors.New("turn log needs a database pool")
	}
	if _, err := pool.Exec(ctx, turnLogSchema); err != nil {
		return nil, fmt.Errorf("create turn log schema: %w", redactDSN(err))
	}
	return &PostgresTurnLog{pool: pool}, nil
}

func (l *PostgresTurnLog) Begin(record TurnRecord) error {
	if strings.TrimSpace(record.MessageID) == "" {
		return errors.New("a turn record needs the message it answers")
	}
	// A redelivered summon is the same turn rather than a second one, so the
	// key collides by design.
	const write = `
INSERT INTO turns (message_id, channel_id, author, process, started_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (message_id) DO UPDATE
SET channel_id = EXCLUDED.channel_id,
    author     = EXCLUDED.author,
    process    = EXCLUDED.process,
    started_at = EXCLUDED.started_at`
	_, err := l.pool.Exec(
		context.Background(), write,
		record.MessageID, record.ChannelID, record.Author, record.Process, record.StartedAt,
	)
	if err != nil {
		return fmt.Errorf("write turn record: %w", redactDSN(err))
	}
	return nil
}

func (l *PostgresTurnLog) Finish(messageID string) error {
	_, err := l.pool.Exec(
		context.Background(), `DELETE FROM turns WHERE message_id = $1`, messageID,
	)
	if err != nil {
		return fmt.Errorf("clear turn record: %w", redactDSN(err))
	}
	return nil
}

func (l *PostgresTurnLog) Sweep(process string) ([]TurnRecord, error) {
	// Taken and cleared in one statement, so two pods booting together cannot
	// both report the same interrupted turn.
	const take = `
DELETE FROM turns WHERE process <> $1
RETURNING message_id, channel_id, author, process, started_at`
	rows, err := l.pool.Query(context.Background(), take, process)
	if err != nil {
		return nil, fmt.Errorf("sweep turn records: %w", redactDSN(err))
	}
	defer rows.Close()
	swept := make([]TurnRecord, 0)
	for rows.Next() {
		var record TurnRecord
		if err := rows.Scan(
			&record.MessageID, &record.ChannelID,
			&record.Author, &record.Process, &record.StartedAt,
		); err != nil {
			return nil, fmt.Errorf("read turn record: %w", redactDSN(err))
		}
		swept = append(swept, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read turn records: %w", redactDSN(err))
	}
	sortRecords(swept)
	return swept, nil
}

// newProcessID names one run of this process. It is compared, never parsed, so
// randomness is the whole requirement.
func newProcessID() string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		// A clock is a worse identity than random bytes and a better one than
		// a fixed string, which would read every record as this run's own.
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

// openTurnLog follows the store the deployment already chose rather than taking
// a second name for one decision. See docs/sirens-echo-execution.md.
func openTurnLog(cfg Config, store JobStore) (TurnLog, error) {
	switch backing := store.(type) {
	case *PostgresJobStore:
		return OpenPostgresTurnLog(context.Background(), backing.pool)
	case *FileJobStore:
		return OpenFileTurnLog(filepath.Join(cfg.JobStoreDir, "turns"))
	default:
		return NewMemoryTurnLog(), nil
	}
}
