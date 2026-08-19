package community

import (
	"strings"
	"testing"
)

// A malformed declaration must fail here rather than at a caller's first use.
func TestEveryDeclaredCommandIsValid(t *testing.T) {
	t.Parallel()
	names := make(map[string]struct{})
	for _, command := range JobCommands(nil) {
		if err := command.Validate(); err != nil {
			t.Errorf("command %s: %v", command.Name, err)
		}
		if _, duplicate := names[command.Name]; duplicate {
			t.Errorf("command %s is declared twice", command.Name)
		}
		names[command.Name] = struct{}{}
	}
	if _, err := discordCommands(nil); err != nil {
		t.Errorf("rendering for Discord: %v", err)
	}
}

// The schema is the authority on arguments, so an undeclared one is refused
// rather than ignored. Ignoring lets a caller believe it took effect. See #147.
func TestUndeclaredArgumentsAreRefusedNotIgnored(t *testing.T) {
	t.Parallel()
	command, ok := LookupCommand("echo", nil)
	if !ok {
		t.Fatal("echo is not declared")
	}
	_, err := command.BindArguments(map[string]string{
		"note":   "fine",
		"target": "production",
	})
	if err == nil {
		t.Fatal("an undeclared argument was accepted")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Errorf("refusal does not name the undeclared argument: %v", err)
	}
}

func TestArgumentBoundsAreEnforced(t *testing.T) {
	t.Parallel()
	command := CommandDefinition{
		Name:        "bounded",
		Description: "for the test",
		Parameters: []CommandParameter{
			{Name: "short", Type: ParameterString, MaxLength: 4},
			{Name: "count", Type: ParameterInteger},
			{Name: "flag", Type: ParameterBoolean},
			{Name: "pick", Type: ParameterString, Choices: []string{"one", "two"}},
			{Name: "job", Type: ParameterString, Pattern: `^job-[a-z0-9]+$`},
			{Name: "must", Type: ParameterString, Required: true},
		},
	}
	if err := command.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	refused := []map[string]string{
		{"must": "yes", "short": "far too long"},
		{"must": "yes", "count": "not-a-number"},
		{"must": "yes", "flag": "maybe"},
		{"must": "yes", "pick": "three"},
		{"must": "yes", "job": "../etc/passwd"},
		{"short": "ok"},
	}
	for _, arguments := range refused {
		if _, err := command.BindArguments(arguments); err == nil {
			t.Errorf("accepted out-of-bounds arguments %v", arguments)
		}
	}
	bound, err := command.BindArguments(map[string]string{
		"must":  "yes",
		"short": "abcd",
		"count": "-12",
		"flag":  "true",
		"pick":  "two",
		"job":   "job-abcdef01",
	})
	if err != nil {
		t.Fatalf("BindArguments: %v", err)
	}
	if len(bound) != 6 {
		t.Errorf("bound = %v", bound)
	}
}

// A command that submits a job must name a kind the harness declares, so the
// command surface cannot outgrow the job surface.
func TestACommandCannotSubmitAnUndeclaredKind(t *testing.T) {
	t.Parallel()
	command := CommandDefinition{
		Name:        "rogue",
		Description: "for the test",
		Kind:        "deploy-everything",
	}
	if err := command.Validate(); err == nil {
		t.Error("a command naming an undeclared job kind validated")
	}
}

func TestThreadBindingIsExplicitAndSingular(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	first := submitTestJob(t, store)

	bound, err := BindJobToThread(store, first.ID, "thread-1")
	if err != nil {
		t.Fatalf("BindJobToThread: %v", err)
	}
	if bound.Origin.ThreadID != "thread-1" {
		t.Fatalf("binding not recorded: %#v", bound.Origin)
	}
	// The binding lives on the record, so resolving is a lookup rather than an
	// inference from recent history.
	resolved, ok, err := ResolveThreadJob(store, "thread-1")
	if err != nil || !ok {
		t.Fatalf("ResolveThreadJob: %v, found=%v", err, ok)
	}
	if resolved.ID != first.ID {
		t.Errorf("resolved %s, want %s", resolved.ID, first.ID)
	}

	other := discordSubmission()
	other.Origin.MessageID = "1537024279743434777"
	prepared, err := PrepareJob(other)
	if err != nil {
		t.Fatalf("PrepareJob: %v", err)
	}
	second, _, err := store.Submit(prepared)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := BindJobToThread(store, second.ID, "thread-1"); err == nil {
		t.Error("a thread was bound to a second job")
	}
	if _, err := BindJobToThread(store, first.ID, "thread-2"); err == nil {
		t.Error("a job was repointed at another thread")
	}
	// Rebinding the same pair is idempotent, which keeps a retry safe.
	if _, err := BindJobToThread(store, first.ID, "thread-1"); err != nil {
		t.Errorf("rebinding the same pair failed: %v", err)
	}
}

// A follow-up in a bound thread resolves without repeating an id.
func TestJobReferenceFallsBackToTheThreadBinding(t *testing.T) {
	t.Parallel()
	store := NewMemoryJobStore(nil)
	job := submitTestJob(t, store)
	if _, err := BindJobToThread(store, job.ID, "thread-9"); err != nil {
		t.Fatalf("BindJobToThread: %v", err)
	}
	id, err := ResolveJobReference(store, "", "thread-9")
	if err != nil {
		t.Fatalf("ResolveJobReference: %v", err)
	}
	if id != job.ID {
		t.Errorf("resolved %s, want %s", id, job.ID)
	}
	// An explicit id wins over the binding.
	if id, err := ResolveJobReference(store, "job-explicit1", "thread-9"); err != nil || id != "job-explicit1" {
		t.Errorf("explicit id = %q, err = %v", id, err)
	}
	// Outside a bound thread with no id, the command has no referent.
	if _, err := ResolveJobReference(store, "", "thread-unbound"); err == nil {
		t.Error("resolved a job with neither an id nor a binding")
	}
}
