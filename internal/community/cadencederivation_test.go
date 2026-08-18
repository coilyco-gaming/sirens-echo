package community

import "testing"

// Three independent numbers that had to agree became one. See sirens-echo#354.

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
