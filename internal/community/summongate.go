package community

// The summon gate is the first of the admission checks and was the only one
// whose refusals were invisible. See docs/sirens-echo-admission.md.

// summonReason is the closed set of summon-gate outcomes. It doubles as a
// metric label, so it never carries member-supplied text.
type summonReason string

const (
	summonDirect      summonReason = "direct_message"
	summonMentioned   summonReason = "mentioned"
	summonOwnedThread summonReason = "owned_thread"
	summonRepliedTo   summonReason = "replied_to"
	// summonNotAddressedThread is separate because it is the one members report
	// and it is sirens-echo#750 rather than a turn that died.
	summonNotAddressedThread summonReason = "not_addressed_in_thread"
	summonNotAddressed       summonReason = "not_addressed"
	summonReplyToAnother     summonReason = "reply_to_another"
	// summonReferenceUnknown is a reply whose referenced message could not be
	// fetched, which is a different thing from one addressed elsewhere.
	summonReferenceUnknown summonReason = "reference_unresolved"
)

// summoned reports whether the reason admitted the message.
func (r summonReason) summoned() bool {
	switch r {
	case summonDirect, summonMentioned, summonOwnedThread, summonRepliedTo:
		return true
	}
	return false
}
