package community

import (
	"testing"
	"time"
)

// Three independent numbers that had to agree became one. See sirens-echo#354.

// The expected cadence, in one place, so a raise edits one file rather than
// three that drift apart. See sirens-echo#901.
var (
	wantProgressAfter  = 10 * time.Second
	wantProgressEvery  = 20 * time.Second
	wantLongReplyAfter = 50 * time.Second
)

// The values, pinned so a change to the base reports what it moved rather than
// moving three things quietly. Raised from 8s in 5806c24.
func TestTheCadenceIsTenTwentyFifty(t *testing.T) {
	t.Parallel()
	for _, check := range []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"startup wait", turnProgressAfter, wantProgressAfter},
		{"artificial delay", turnProgressEvery, wantProgressEvery},
		{"long reply window", turnLongReplyAfter, wantLongReplyAfter},
	} {
		if check.got != check.want {
			t.Errorf("%s = %s, want %s", check.name, check.got, check.want)
		}
	}
}

// The point of the change is that one edit moves all three. A derivation that
// silently stopped deriving would look identical at today's values.
func TestTheCadenceIsDerivedAndNotThreeCoincidences(t *testing.T) {
	t.Parallel()
	if turnProgressEvery != turnProgressAfter*2 {
		t.Errorf("the beat is no longer twice the wait: %s vs %s", turnProgressEvery, turnProgressAfter)
	}
	if turnLongReplyAfter != turnProgressAfter+turnProgressEvery*2 {
		t.Errorf("the window is no longer the wait plus two beats: %s", turnLongReplyAfter)
	}
	// A window shorter than the first beat would mean every turn that posted a
	// progress line was already long, which is not what long means.
	if turnLongReplyAfter <= turnProgressAfter {
		t.Error("the long-reply window opens before the progress line does")
	}
}
