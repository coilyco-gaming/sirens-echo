package community

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// promptBudgets ratchets the system prompt. The turn context is the other half
// of every request and nothing measured it. See docs/sirens-echo-turn-cost.md.

// turnContextBudget is the worst case a turn may assemble, in bytes. Raising N
// or a per-entry cap moves this, which is the point: it is paid every turn.
const turnContextBudget = 16000

// worstCaseTurn builds the largest turn the caps permit at the tracked window.
func worstCaseTurn(t *testing.T, window int) TurnPrompt {
	t.Helper()
	history := make([]TranscriptEntry, 0, window)
	for range window {
		history = append(history, TranscriptEntry{
			Author:  strings.Repeat("a", 200),
			Content: strings.Repeat("c", 4000),
		})
	}
	return BuildTurnPrompt(
		"",
		history,
		TranscriptEntry{
			Author:  strings.Repeat("a", 200),
			Content: strings.Repeat("m", 8000),
		},
	)
}

// The inputs are deliberately larger than the caps, so this measures what the
// caps allow rather than what a caller happened to send.
func TestTurnContextStaysInsideItsBudget(t *testing.T) {
	t.Parallel()
	window := trackedContextWindow(t)
	prompt := worstCaseTurn(t, window)
	size := len(prompt.Context) + len(prompt.Message)
	if size > turnContextBudget {
		t.Fatalf(
			"a worst-case turn assembles %d bytes against a %d byte budget at a "+
				"window of %d. Raise turnContextBudget and say why, or lower "+
				"max_context_messages or the per-entry caps",
			size, turnContextBudget, window,
		)
	}
}

// A cap that stopped truncating would not fail the budget on its own, because
// the budget is a ceiling rather than an equality.
func TestTranscriptCapsStillTruncate(t *testing.T) {
	t.Parallel()
	prompt := worstCaseTurn(t, 1)
	if !strings.Contains(prompt.Context, "…") {
		t.Error("an oversized history entry was not truncated, so the per-entry cap is gone")
	}
	if len([]rune(prompt.Message)) > 2001 {
		t.Errorf("the current message rendered %d runes, so its cap is gone",
			len([]rune(prompt.Message)))
	}
}

// The budget means nothing if it is measured at a smaller window than the one
// the deployment actually uses, so the window comes from the tracked file.
func trackedContextWindow(t *testing.T) int {
	t.Helper()
	definitions, err := filepath.Glob(filepath.Join("..", "..", "agent", "*.yaml"))
	if err != nil || len(definitions) == 0 {
		t.Fatalf("glob agent definitions: %v, found %d", err, len(definitions))
	}
	widest := 0
	for _, path := range definitions {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		value := yamlScalar(string(body), "max_context_messages")
		if value == "" {
			continue
		}
		window, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("%s max_context_messages is %q", filepath.Base(path), value)
		}
		if window > widest {
			widest = window
		}
	}
	if widest == 0 {
		t.Fatal("no agent definition declared max_context_messages")
	}
	return widest
}
