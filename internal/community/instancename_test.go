package community

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fallback for an unset instance name is a live service, so a profile that
// forgets it reports as Echo rather than as misconfigured. See sirens-echo#542.

// This is the shape Echo's own deployment ships: values.yaml sets neither
// SIRENS_ECHO_DEFINITION nor SIRENS_ECHO_INSTANCE. Breaking it renames Echo.
func TestEchoStillGetsItsNameFromNeitherVariable(t *testing.T) {
	t.Parallel()
	name, err := resolveInstanceName(defaultInstanceIdentity, "")
	if err != nil {
		t.Fatalf("Echo's own shipped configuration was rejected: %v", err)
	}
	if name != defaultInstanceName {
		t.Errorf("Echo resolved to %q, want %q", name, defaultInstanceName)
	}
}

// Echo's definition by another path is still Echo's. Keying on identity makes
// that structural, so it is asserted where paths enter: LoadConfig.
func TestEchosDefinitionIsRecognisedByAnyPath(t *testing.T) {
	t.Setenv("SIRENS_ECHO_DEFINITION", filepath.Join("..", "..", "agent", "sirens-echo.yaml"))
	t.Setenv("SIRENS_ECHO_BUNDLE_DIR", writeFixtureBundle(t))
	t.Setenv("SIRENS_ECHO_DISCORD_ENABLED", "false")
	t.Setenv("AGENT_PROXY_MODEL", "model")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Echo's definition by a relative path was refused: %v", err)
	}
	if cfg.InstanceName != defaultInstanceName {
		t.Errorf("InstanceName = %q, want %q", cfg.InstanceName, defaultInstanceName)
	}
}

// The defect: 891 spans from a Deep profile reported as service.name
// sirens-echo, because the default applied to a definition that is not Echo's.
func TestANonEchoDefinitionCannotDefaultToEchosName(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"Sirens Deep of Coilyco",
		"Coilyco General",
	} {
		name, err := resolveInstanceName(path, "")
		if err == nil {
			t.Errorf("%s resolved to %q instead of refusing", path, name)
			continue
		}
		// The message has to name the variable to set, or the operator reading a
		// crash loop learns only that it crashed.
		if !strings.Contains(err.Error(), "SIRENS_ECHO_INSTANCE") {
			t.Errorf("%s: the reason does not name the variable: %v", path, err)
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("%s: the reason does not name the definition: %v", path, err)
		}
	}
}

// Deep in production sets both, which must keep working unchanged.
func TestAConfiguredNameIsUsedForAnyDefinition(t *testing.T) {
	t.Parallel()
	for definition, configured := range map[string]string{
		"Sirens Deep of Coilyco": "sirens-deep",
		defaultInstanceIdentity:  "sirens-echo-canary",
	} {
		name, err := resolveInstanceName(definition, configured)
		if err != nil {
			t.Errorf("%s with %q: %v", definition, configured, err)
			continue
		}
		if name != configured {
			t.Errorf("%s resolved to %q, want %q", definition, name, configured)
		}
	}
}

// A variable set to spaces is unset. Treating it as a name would ship a service
// called "   " rather than fail, which is the same class of silence.
func TestAWhitespaceNameIsNotAName(t *testing.T) {
	t.Parallel()
	name, err := resolveInstanceName(defaultInstanceIdentity, "   ")
	if err != nil {
		t.Fatalf("Echo with a blank override was rejected: %v", err)
	}
	if name != defaultInstanceName {
		t.Errorf("a blank override produced %q", name)
	}
	if _, err := resolveInstanceName("Sirens Deep of Coilyco", "  \t "); err == nil {
		t.Error("a blank override satisfied the requirement for a non-Echo definition")
	}
}

// Surrounding space in a real name is trimmed, because service.name is compared
// literally in every query that reads it.
func TestAConfiguredNameIsTrimmed(t *testing.T) {
	t.Parallel()
	name, err := resolveInstanceName("Sirens Deep of Coilyco", "  sirens-deep\n")
	if err != nil {
		t.Fatalf("resolveInstanceName: %v", err)
	}
	if name != "sirens-deep" {
		t.Errorf("name = %q, want %q", name, "sirens-deep")
	}
}

// A filename is not an identity: Deep's content in a file called
// sirens-echo.yaml used to take Echo's service name. See sirens-echo#706.
func TestAForeignDefinitionInAnEchoNamedFileIsRefused(t *testing.T) {
	deep, err := os.ReadFile(filepath.Join("..", "..", "agent", "sirens-deep.yaml"))
	if err != nil {
		t.Fatalf("read Deep's definition: %v", err)
	}
	decoy := filepath.Join(t.TempDir(), filepath.Base(defaultDefinitionPath))
	if err := os.WriteFile(decoy, deep, 0o600); err != nil {
		t.Fatalf("write decoy: %v", err)
	}

	t.Setenv("SIRENS_ECHO_DEFINITION", decoy)
	t.Setenv("SIRENS_ECHO_BUNDLE_DIR", writeFixtureBundle(t))
	t.Setenv("SIRENS_ECHO_DISCORD_ENABLED", "false")
	t.Setenv("AGENT_PROXY_MODEL", "model")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("Deep's definition took Echo's service name because the file was named sirens-echo.yaml")
	}
}
