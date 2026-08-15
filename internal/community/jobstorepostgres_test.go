package community

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The selection is the whole deployment contract: which of three stores a set
// of environment variables produces.
func TestTheJobStoreSelectionFollowsTheConfig(t *testing.T) {
	t.Parallel()

	memory, err := openJobStore(Config{})
	if err != nil {
		t.Fatalf("unconfigured: %v", err)
	}
	if _, ok := memory.(*MemoryJobStore); !ok {
		t.Errorf("unconfigured store = %T, want the memory store", memory)
	}

	file, err := openJobStore(Config{JobStoreDir: filepath.Join(t.TempDir(), "jobs")})
	if err != nil {
		t.Fatalf("directory: %v", err)
	}
	if _, ok := file.(*FileJobStore); !ok {
		t.Errorf("directory store = %T, want the file store", file)
	}
}

// Both set is refused rather than resolved. Picking one silently would put the
// jobs somewhere nobody was watching. See docs/sirens-echo-jobs-store.md.
func TestABothStoresConfiguredDeploymentIsRefused(t *testing.T) {
	t.Parallel()
	_, err := openJobStore(Config{
		JobStoreDir: "/var/lib/sirens-echo/jobs",
		JobStoreDSN: "host=sirens-echo-job-store user=sirens password=secret dbname=sirens_jobs",
	})
	if err == nil {
		t.Fatal("accepted a deployment that configured two job stores")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("the refusal carried the password: %v", err)
	}
}

// The DSN is a credential, so a connection failure must not print it. pgx puts
// the config into some failure strings, which is what this guards.
func TestTheDSNPasswordNeverReachesAnError(t *testing.T) {
	t.Parallel()
	// Assembled rather than written out, so no credential-shaped literal is
	// tracked. pgx hands redactDSN the URL form, which is what this mimics.
	raw := fmt.Errorf("failed to connect to %s://%s:%s@%s",
		"postgres", "sirens", "hunter2", "sirens-echo-job-store:5432/sirens_jobs")
	got := redactDSN(raw).Error()
	if strings.Contains(got, "hunter2") {
		t.Errorf("redactDSN kept the password: %s", got)
	}
	if !strings.Contains(got, "sirens-echo-job-store") {
		t.Errorf("redactDSN dropped the host, which a reader needs: %s", got)
	}
}

// An empty DSN is a configuration error rather than a dial that fails later.
func TestAnEmptyDSNIsRefusedBeforeDialing(t *testing.T) {
	t.Parallel()
	if _, err := OpenPostgresJobStore(context.Background(), "   ", nil); err == nil {
		t.Fatal("accepted an empty DSN")
	}
}

// postgresTestStore opens the store against a real database, or skips. There is
// no in-process Postgres, so the alternative is no SQL coverage at all.
func postgresTestStore(t *testing.T) *PostgresJobStore {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("SIRENS_ECHO_TEST_JOB_STORE_DSN"))
	if dsn == "" {
		t.Skip("set SIRENS_ECHO_TEST_JOB_STORE_DSN to a scratch database to exercise the SQL")
	}
	store, err := OpenPostgresJobStore(context.Background(), dsn, fixedClock(time.Unix(1700000000, 0).UTC()))
	if err != nil {
		t.Fatalf("OpenPostgresJobStore: %v", err)
	}
	t.Cleanup(func() {
		if _, err := store.pool.Exec(context.Background(), `DROP TABLE IF EXISTS jobs`); err != nil {
			t.Errorf("drop scratch table: %v", err)
		}
		store.Close()
	})
	return store
}

// The point of the whole change: a record written before a restart is still
// there after one, on a store that outlives the pod.
func TestThePostgresStoreSurvivesAReopen(t *testing.T) {
	store := postgresTestStore(t)
	job := submitTestJob(t, store)
	if _, err := store.Transition(job.ID, JobRunning, nil); err != nil {
		t.Fatalf("to running: %v", err)
	}

	reopened, err := OpenPostgresJobStore(
		context.Background(),
		os.Getenv("SIRENS_ECHO_TEST_JOB_STORE_DSN"),
		fixedClock(time.Unix(1700001000, 0).UTC()),
	)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	recovered, err := reopened.Get(job.ID)
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if recovered.State != JobRunning {
		t.Errorf("state after restart = %s, want running", recovered.State)
	}
	if len(reopened.All()) != 1 {
		t.Errorf("All() found %d jobs, want the one that outlived the restart", len(reopened.All()))
	}
}

// A redelivered submission collapses onto the first job rather than making a
// second one, which is what the unique key on the record buys.
func TestThePostgresStoreCollapsesARedeliveredSubmission(t *testing.T) {
	store := postgresTestStore(t)
	prepared, err := PrepareJob(discordSubmission())
	if err != nil {
		t.Fatalf("PrepareJob: %v", err)
	}
	first, existed, err := store.Submit(prepared)
	if err != nil || existed {
		t.Fatalf("first submit: job=%v existed=%v err=%v", first.ID, existed, err)
	}
	again, existed, err := store.Submit(prepared)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if !existed {
		t.Error("a redelivered submission made a second job")
	}
	if again.ID != first.ID {
		t.Errorf("redelivery returned %s, want the original %s", again.ID, first.ID)
	}
}

// A refused move leaves the stored record untouched. The read and the write are
// two statements here, so this is the behaviour the row lock is buying.
func TestThePostgresStoreLeavesARefusedMoveAlone(t *testing.T) {
	store := postgresTestStore(t)
	job := submitTestJob(t, store)
	if _, err := store.Transition(job.ID, JobSucceeded, nil); err == nil {
		t.Fatal("queued moved straight to succeeded")
	}
	current, err := store.Get(job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if current.State != JobQueued {
		t.Errorf("state after a refused move = %s, want queued", current.State)
	}
}

// ListByThread reads a projected column, and Update rebinds the thread. A
// projection that only tracked inserts would answer from a stale value.
func TestThePostgresStoreFindsAJobReboundToAThread(t *testing.T) {
	store := postgresTestStore(t)
	job := submitTestJob(t, store)
	if _, err := BindJobToThread(store, job.ID, "1537024279743434999"); err != nil {
		t.Fatalf("BindJobToThread: %v", err)
	}
	found, err := store.ListByThread("1537024279743434999")
	if err != nil {
		t.Fatalf("ListByThread: %v", err)
	}
	if len(found) != 1 || found[0].ID != job.ID {
		t.Errorf("ListByThread found %d jobs, want the rebound one", len(found))
	}
}

// An unknown id is ErrJobNotFound rather than a driver error, because callers
// branch on it.
func TestThePostgresStoreReportsAMissingJob(t *testing.T) {
	store := postgresTestStore(t)
	if _, err := store.Get("job-does-not-exist"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("Get of an unknown id = %v, want ErrJobNotFound", err)
	}
	if _, err := store.Transition("job-does-not-exist", JobRunning, nil); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("Transition of an unknown id = %v, want ErrJobNotFound", err)
	}
}
