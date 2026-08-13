package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The gate deploy cannot build itself. See sirens-echo#628.

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "access-policy.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

const boundedGuild = `schema: coilyco-harness.access.v1
guilds:
  - id: "111111111111111111"
    channels: ["222222222222222222"]
    users: ["333333333333333333"]
`

func TestABoundedPolicyPasses(t *testing.T) {
	t.Parallel()
	if err := check(write(t, boundedGuild)); err != nil {
		t.Fatalf("a valid policy was rejected: %v", err)
	}
}

// The check the issue names as the one that matters: an open guild without a
// per-user bound is an unbounded guild.
func TestAnOpenGuildWithoutAPerUserBoundFails(t *testing.T) {
	t.Parallel()
	err := check(write(t, `schema: coilyco-harness.access.v1
guilds:
  - id: "111111111111111111"
    channels: ["222222222222222222"]
    users: all
`))
	if err == nil {
		t.Fatal("an open guild with no per-user rate limit was accepted")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "rate") {
		t.Errorf("the reason does not name the missing bound: %v", err)
	}
}

func TestAMissingFileIsAnError(t *testing.T) {
	t.Parallel()
	if err := check(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("a missing policy file was accepted")
	}
}

// Strict decoding is the property that catches a typo'd key, which is the
// failure mode a plain YAML parse in another repository cannot see.
func TestAnUnknownFieldIsAnError(t *testing.T) {
	t.Parallel()
	err := check(write(t, `schema: coilyco-harness.access.v1
guilds:
  - id: "111111111111111111"
    channels: ["222222222222222222"]
    users: ["333333333333333333"]
    ratelimit: {}
`))
	if err == nil {
		t.Fatal("a misspelled key was accepted, so a typo disables a bound silently")
	}
}

func TestAnUnsupportedSchemaIsAnError(t *testing.T) {
	t.Parallel()
	err := check(write(t, `schema: something.else.v9
guilds:
  - id: "111111111111111111"
    channels: ["222222222222222222"]
    users: ["333333333333333333"]
`))
	if err == nil {
		t.Fatal("an unsupported schema was accepted")
	}
}
