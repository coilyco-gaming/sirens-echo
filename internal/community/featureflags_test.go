package community

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The numbers got one file, one helper, a validator, and a generated
// reference. The switches now get the same. See sirens-echo#854.

// A switch read anywhere else is a switch nobody can enumerate, which is the
// whole reason one table holds them.
func TestEveryFeatureFlagLivesInTheTable(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	strays := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || name == "featureflags.go" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for index, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "boolOrDefault(") &&
				!strings.Contains(line, "func boolOrDefault(") {
				strays = append(strays, name+":"+itoa(index+1))
			}
		}
	}
	sort.Strings(strays)
	if len(strays) > 0 {
		t.Errorf("a feature switch is parsed outside the table, so the generated "+
			"reference does not list it. Add it to featureFlags:\n%s",
			strings.Join(strays, "\n"))
	}
}

// A flag in the table with nothing behind it, or two sharing a name, would make
// the reference wrong in a way reading it cannot reveal.
func TestTheFlagTableIsWellFormed(t *testing.T) {
	t.Parallel()
	var cfg Config
	seen := make(map[string]bool)
	targets := make(map[*bool]bool)
	for _, flag := range featureFlags(&cfg) {
		if !strings.HasPrefix(flag.env, "SIRENS_ECHO_") {
			t.Errorf("flag %q is outside the SIRENS_ECHO_ namespace", flag.env)
		}
		if seen[flag.env] {
			t.Errorf("flag %q appears twice", flag.env)
		}
		seen[flag.env] = true
		if flag.target == nil {
			t.Errorf("flag %q binds no field", flag.env)
			continue
		}
		if targets[flag.target] {
			t.Errorf("flag %q shares a field with another flag", flag.env)
		}
		targets[flag.target] = true
		if strings.TrimSpace(flag.summary) == "" {
			t.Errorf("flag %q says nothing about what it turns on", flag.env)
		}
	}
	if len(seen) == 0 {
		t.Fatal("the flag table is empty, so this test asserts nothing")
	}
}

// Defaults and overrides both reach the field, and a value that does not parse
// is fatal and named rather than silently taken as the default.
func TestApplyFeatureFlagsReadsTheEnvironment(t *testing.T) {
	t.Parallel()
	var defaults Config
	if err := applyFeatureFlags(&defaults, func(string) string { return "" }); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if !defaults.DiscordEnabled {
		t.Error("the Discord surface defaults off")
	}
	if defaults.DiscordDMEnabled || defaults.DiscordCommandsEnabled {
		t.Error("a surface with no guild moderation behind it defaults on")
	}

	var overridden Config
	err := applyFeatureFlags(&overridden, func(name string) string {
		if name == "SIRENS_ECHO_DISCORD_COMMANDS" {
			return "true"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if !overridden.DiscordCommandsEnabled {
		t.Error("an override did not reach the field")
	}

	var broken Config
	err = applyFeatureFlags(&broken, func(name string) string {
		if name == "SIRENS_ECHO_DISCORD_DM_ENABLED" {
			return "yes please"
		}
		return ""
	})
	if err == nil {
		t.Fatal("an unparsable switch was taken as its default")
	}
	if !strings.Contains(err.Error(), "SIRENS_ECHO_DISCORD_DM_ENABLED") {
		t.Errorf("the error does not name the switch: %v", err)
	}
}

// The reference is the thing a deployment reads, so every flag has to reach it.
func TestTheFlagReferenceListsEveryFlag(t *testing.T) {
	t.Parallel()
	var cfg Config
	rendered := RenderFlagReference()
	if !strings.HasPrefix(rendered, FlagReferenceHeading) {
		t.Errorf("the reference does not open with its heading: %q", rendered)
	}
	for _, flag := range featureFlags(&cfg) {
		short := strings.TrimPrefix(flag.env, "SIRENS_ECHO_")
		if !strings.Contains(rendered, short) {
			t.Errorf("%s is missing from the reference", short)
		}
		if !strings.Contains(rendered, flag.summary) {
			t.Errorf("%s reaches the reference without saying what it turns on", short)
		}
	}
}

// Generated from the table, so it cannot fall behind what the code offers.
func TestTheFlagReferenceIsCurrent(t *testing.T) {
	t.Parallel()
	tracked, err := os.ReadFile(filepath.Join("..", "..", "agent", "rendered", "flags.txt"))
	if err != nil {
		t.Fatalf("read the reference: %v", err)
	}
	if string(tracked) != RenderFlagReference() {
		t.Error("agent/rendered/flags.txt is stale. Run `just flags`.")
	}
}

// A deployment reads the prose page first, so it has to reach the list from
// there rather than from the source.
func TestTheTuningDocPointsAtTheFlagReference(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "sirens-echo-tuning.md"))
	if err != nil {
		t.Fatalf("read the tuning doc: %v", err)
	}
	if !strings.Contains(string(body), "agent/rendered/flags.txt") {
		t.Error("the tuning doc never links the generated feature reference, so a " +
			"deployment cannot find the list of switches")
	}
}
