package community

import (
	"strings"
	"testing"
)

// Naming someone should reach them, and only people already in the
// conversation can be reached. See sirens-echo#219.

func conversationRoster() mentionRoster {
	roster := mentionRoster{}
	roster.add("coilysiren", "318190481467244544")
	roster.add("alpha", "401110000000000001")
	return roster
}

func TestNamingSomeoneResolvesToAMention(t *testing.T) {
	t.Parallel()
	reply, mentioned := conversationRoster().resolveMentions("alpha raised this earlier.")
	if !strings.Contains(reply, "<@401110000000000001>") {
		t.Errorf("reply carries no mention: %q", reply)
	}
	if len(mentioned) != 1 || mentioned[0] != "401110000000000001" {
		t.Errorf("resolved = %v, want the one account named", mentioned)
	}
}

// A reply naming someone four times should reach them once. Four pings for one
// sentence is the behaviour that makes people mute a channel.
func TestSomeoneNamedFourTimesIsReachedOnce(t *testing.T) {
	t.Parallel()
	reply, mentioned := conversationRoster().resolveMentions(
		"alpha asked, alpha followed up, alpha waited, and alpha is still waiting.",
	)
	if got := strings.Count(reply, "<@401110000000000001>"); got != 1 {
		t.Errorf("mentioned %d times, want 1: %q", got, reply)
	}
	if len(mentioned) != 1 {
		t.Errorf("resolved %d accounts, want 1", len(mentioned))
	}
	// The other three stay as the name, which still reads correctly.
	if strings.Count(reply, "alpha") != 3 {
		t.Errorf("the remaining names were altered: %q", reply)
	}
}

// Nobody outside the conversation is reachable, which is the whole bound.
func TestSomeoneNotInTheConversationIsNotReached(t *testing.T) {
	t.Parallel()
	reply, mentioned := conversationRoster().resolveMentions("Abhay would know this.")
	if strings.Contains(reply, "<@") {
		t.Errorf("someone outside the transcript was mentioned: %q", reply)
	}
	if len(mentioned) != 0 {
		t.Errorf("resolved %v for a name nobody in the turn holds", mentioned)
	}
}

// A name already written as a mention must not be rewritten into a nested one.
func TestAnExistingMentionIsLeftAlone(t *testing.T) {
	t.Parallel()
	reply, _ := conversationRoster().resolveMentions("<@318190481467244544> already answered.")
	if strings.Contains(reply, "<@<@") || strings.Count(reply, "<@") != 1 {
		t.Errorf("an existing mention was rewritten: %q", reply)
	}
}

// A short name matches too much ordinary prose to be safe.
func TestATooShortNameIsNeverResolved(t *testing.T) {
	t.Parallel()
	roster := mentionRoster{}
	roster.add("jo", "401110000000000002")
	roster.add("a", "401110000000000003")
	if len(roster) != 0 {
		t.Errorf("a short name entered the roster: %v", roster)
	}
	reply, mentioned := roster.resolveMentions("a job is not a person")
	if strings.Contains(reply, "<@") || len(mentioned) != 0 {
		t.Errorf("a short name was resolved: %q %v", reply, mentioned)
	}
}

// A name inside a longer word is not that person.
func TestANameInsideAWordIsNotAPerson(t *testing.T) {
	t.Parallel()
	reply, mentioned := conversationRoster().resolveMentions("the alphabet came before alpha")
	if !strings.Contains(reply, "alphabet came") {
		t.Errorf("the word alphabet was broken: %q", reply)
	}
	if !strings.HasSuffix(reply, "<@401110000000000001>") {
		t.Errorf("the standalone name did not resolve: %q", reply)
	}
	if len(mentioned) != 1 {
		t.Errorf("resolved %v, want exactly the standalone name", mentioned)
	}
}

// A longer display name containing a shorter one resolves as itself.
func TestTheLongerNameWins(t *testing.T) {
	t.Parallel()
	roster := mentionRoster{}
	roster.add("alpha", "401110000000000001")
	roster.add("alpha centauri", "401110000000000004")
	reply, _ := roster.resolveMentions("alpha centauri filed it")
	if !strings.Contains(reply, "<@401110000000000004>") {
		t.Errorf("the longer name did not win: %q", reply)
	}
}

// An empty roster is the ordinary case and must leave a reply byte-identical.
func TestAnEmptyRosterChangesNothing(t *testing.T) {
	t.Parallel()
	reply := "The Eco server is online."
	got, mentioned := mentionRoster{}.resolveMentions(reply)
	if got != reply || len(mentioned) != 0 {
		t.Errorf("an empty roster altered the reply: %q %v", got, mentioned)
	}
}
