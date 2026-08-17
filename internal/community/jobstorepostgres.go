package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresJobStore keeps the record in a database, so durability outlives the
// pod rather than its volume. See docs/sirens-echo-jobs.md.
type PostgresJobStore struct {
	Now Clock

	pool *pgxpool.Pool
}

// openJobStore picks the backend the deployment configured. Both variables set
// is refused rather than resolved. See docs/sirens-echo-jobs.md.
func openJobStore(cfg Config) (JobStore, error) {
	switch {
	case cfg.JobStoreDSN != "" && cfg.JobStoreDir != "":
		return nil, fmt.Errorf(
			"SIRENS_ECHO_JOB_STORE_DSN and SIRENS_ECHO_JOB_STORE are both set: pick one store",
		)
	case cfg.JobStoreDSN != "":
		return OpenPostgresJobStore(context.Background(), cfg.JobStoreDSN, nil)
	case cfg.JobStoreDir != "":
		return OpenFileJobStore(cfg.JobStoreDir, nil)
	default:
		return NewMemoryJobStore(nil), nil
	}
}

// jobStoreSchema is applied at open. Idempotent because that is the whole
// migration story: one table, created on first boot.
const jobStoreSchema = `
CREATE TABLE IF NOT EXISTS jobs (
    id              TEXT PRIMARY KEY,
    idempotency_key TEXT NOT NULL UNIQUE,
    principal       TEXT NOT NULL,
    thread_id       TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL,
    record          JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS jobs_principal_created
    ON jobs (principal, created_at DESC, id);
CREATE INDEX IF NOT EXISTS jobs_thread_created
    ON jobs (thread_id, created_at DESC, id) WHERE thread_id <> '';
`

// OpenPostgresJobStore dials the database and ensures the schema. It fails
// rather than falling back to memory, which would be silent data loss.
func OpenPostgresJobStore(ctx context.Context, dsn string, now Clock) (*PostgresJobStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("job store DSN is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		// The DSN carries the password, so it never reaches an error string.
		return nil, fmt.Errorf("open job store: %w", redactDSN(err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("reach job store: %w", redactDSN(err))
	}
	if _, err := pool.Exec(ctx, jobStoreSchema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create job store schema: %w", redactDSN(err))
	}
	return &PostgresJobStore{Now: now, pool: pool}, nil
}

// Close releases the pool, for a test and for an orderly shutdown.
func (s *PostgresJobStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *PostgresJobStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// redactDSN keeps a connection error from carrying the password, which pgx
// puts into some failure strings.
func redactDSN(err error) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	for {
		start := strings.Index(text, "://")
		if start < 0 {
			break
		}
		at := strings.Index(text[start:], "@")
		if at < 0 {
			break
		}
		text = text[:start+3] + "REDACTED" + text[start+at:]
		break
	}
	return errors.New(text)
}

func (s *PostgresJobStore) Submit(job Job) (Job, bool, error) {
	ctx := context.Background()
	prepared, err := prepareSubmission(job, s.now())
	if err != nil {
		return Job{}, false, err
	}
	raw, err := json.Marshal(prepared)
	if err != nil {
		return Job{}, false, fmt.Errorf("encode job %s: %w", prepared.ID, err)
	}
	// DO NOTHING covers both unique constraints, so the lookup below is what
	// says which one collided.
	tag, err := s.pool.Exec(
		ctx,
		`INSERT INTO jobs (id, idempotency_key, principal, thread_id, created_at, record)
		 VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
		prepared.ID, prepared.IdempotencyKey, prepared.Principal,
		prepared.Origin.ThreadID, prepared.CreatedAt, raw,
	)
	if err != nil {
		return Job{}, false, fmt.Errorf("submit job %s: %w", prepared.ID, err)
	}
	if tag.RowsAffected() == 1 {
		return prepared, false, nil
	}
	existing, err := s.byIdempotencyKey(ctx, prepared.IdempotencyKey)
	if err == nil {
		return existing, true, nil
	}
	if errors.Is(err, ErrJobNotFound) {
		return Job{}, false, fmt.Errorf("job %s already exists under a different key", prepared.ID)
	}
	return Job{}, false, err
}

func (s *PostgresJobStore) byIdempotencyKey(ctx context.Context, key string) (Job, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT record FROM jobs WHERE idempotency_key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, fmt.Errorf("%w: key %s", ErrJobNotFound, key)
	}
	if err != nil {
		return Job{}, fmt.Errorf("read job by key: %w", err)
	}
	return decodeJob(raw)
}

func (s *PostgresJobStore) Get(id string) (Job, error) {
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `SELECT record FROM jobs WHERE id = $1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}
	if err != nil {
		return Job{}, fmt.Errorf("read job %s: %w", id, err)
	}
	return decodeJob(raw)
}

func (s *PostgresJobStore) ListByPrincipal(principal string) ([]Job, error) {
	return s.list(
		`SELECT record FROM jobs WHERE principal = $1 ORDER BY created_at DESC, id ASC`,
		principal,
	)
}

func (s *PostgresJobStore) ListByThread(threadID string) ([]Job, error) {
	if threadID == "" {
		return nil, nil
	}
	return s.list(
		`SELECT record FROM jobs WHERE thread_id = $1 ORDER BY created_at DESC, id ASC`,
		threadID,
	)
}

// All returns every stored job, newest first. Restart recovery reaches it by
// interface assertion, and without it a roll strands every running job.
func (s *PostgresJobStore) All() []Job {
	jobs, err := s.list(`SELECT record FROM jobs ORDER BY created_at DESC, id ASC`)
	if err != nil {
		return nil
	}
	return jobs
}

func (s *PostgresJobStore) list(query string, args ...any) ([]Job, error) {
	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()
	jobs := make([]Job, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		job, err := decodeJob(raw)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	return jobs, nil
}

func (s *PostgresJobStore) Transition(id string, next JobState, mutate func(*Job)) (Job, error) {
	return s.write(id, func(job Job, moment time.Time) (Job, error) {
		return applyTransition(job, next, mutate, moment)
	})
}

func (s *PostgresJobStore) Update(id string, mutate func(*Job)) (Job, error) {
	return s.write(id, func(job Job, moment time.Time) (Job, error) {
		return applyUpdate(job, mutate, moment)
	})
}

// write reads the row under a lock, applies the caller's rule, and persists.
// The lock is what leaves a refused move's record untouched across two statements.
func (s *PostgresJobStore) write(id string, apply func(Job, time.Time) (Job, error)) (Job, error) {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Job{}, fmt.Errorf("write job %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var raw []byte
	err = tx.QueryRow(ctx, `SELECT record FROM jobs WHERE id = $1 FOR UPDATE`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}
	if err != nil {
		return Job{}, fmt.Errorf("read job %s: %w", id, err)
	}
	current, err := decodeJob(raw)
	if err != nil {
		return Job{}, err
	}
	updated, err := apply(current, s.now())
	if err != nil {
		return Job{}, err
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		return Job{}, fmt.Errorf("encode job %s: %w", id, err)
	}
	// Rewritten here and not only on insert: Update rebinds Origin.ThreadID,
	// and an insert-only projection would answer ListByThread from a stale one.
	if _, err := tx.Exec(
		ctx,
		`UPDATE jobs SET record = $2, principal = $3, thread_id = $4 WHERE id = $1`,
		id, encoded, updated.Principal, updated.Origin.ThreadID,
	); err != nil {
		return Job{}, fmt.Errorf("write job %s: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, fmt.Errorf("commit job %s: %w", id, err)
	}
	return updated, nil
}

func decodeJob(raw []byte) (Job, error) {
	var job Job
	if err := json.Unmarshal(raw, &job); err != nil {
		return Job{}, fmt.Errorf("parse stored job: %w", err)
	}
	return job, nil
}
