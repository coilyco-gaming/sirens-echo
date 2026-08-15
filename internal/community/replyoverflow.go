package community

import (
	"context"
	"fmt"
)

// The outbound hop was the one place a large payload was destroyed rather than
// saved. See docs/sirens-echo-reply-overflow.md.

const (
	// overflowNotice names the file and the true size, the way the inbound and
	// tool-result paths already name theirs.
	overflowNotice = "\n[the whole reply is attached as %s, %d bytes]"
	// overflowFileName is fixed, so nothing member-supplied or model-supplied
	// reaches an attachment name.
	overflowFileName = "reply.txt"
	// nothingWithheld is what a send carries when the message is all of it.
	nothingWithheld = ""
)

// overflowCarrier is an optional turn capability, asserted like the reactor: a
// transport that can carry text its message budget cut declares it.
type overflowCarrier interface {
	ReplyWithOverflow(ctx context.Context, content, whole string) error
}

// deliverWithOverflow attaches the whole reply where the transport can carry
// it, and falls back to the cut message where it cannot.
func deliverWithOverflow(ctx context.Context, turn turnIO, content, whole string) error {
	carrier, ok := turn.(overflowCarrier)
	if !ok || whole == nothingWithheld {
		return turn.Reply(ctx, content)
	}
	return carrier.ReplyWithOverflow(ctx, content, whole)
}

// fitWithOverflow returns the message to send and the whole reply that message
// is a prefix of. The second return is empty when nothing was cut.
func fitWithOverflow(
	answer string, limit int, executed []ExecutedTool,
) (content, whole string) {
	whole = AssembleReply(answer, unboundedReply, executed...)
	if limit <= 0 || runeLen(whole) <= limit {
		return whole, nothingWithheld
	}
	notice := fmt.Sprintf(overflowNotice, overflowFileName, len(whole))
	room := limit - runeLen(notice)
	// Too large to attach, or too small a budget to say so. Both fall back to
	// the previous behaviour rather than failing the turn.
	if len(whole) > replyAttachmentBytes || room <= 0 {
		return AssembleReply(answer, limit, executed...), nothingWithheld
	}
	return AssembleReply(answer, room, executed...) + notice, whole
}
