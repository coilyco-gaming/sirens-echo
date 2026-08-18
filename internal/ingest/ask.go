// Package ingest receives community asks, acknowledges each one before any
// batching decision exists, and hands them to the coalescer over a bounded
// queue that sheds rather than blocks.
//
// The demo rule this package exists to keep is an acknowledgment per comment
// with the substantive work batched behind it, so the ack is applied here and
// never by the worker that answers. See docs/sirens-echo-admission.md.
package ingest

import (
	"strings"
	"time"
)

// Surfaces an ask can arrive on. Closed set, because it reaches metric labels
// and a member-supplied value there would be unbounded cardinality.
const (
	SurfaceDiscord = "discord"
	SurfaceSite    = "site"
)

// Tenant is the coalescing shard and the unit of write exclusivity. Guild and
// channel already group a conversation, so the member is the shard that adds.
type Tenant struct {
	Surface string
	Guild   string
	Channel string
	Author  string
}

// Key identifies the shard. The separator is a character no Discord snowflake
// and no site slug contains, so two tenants cannot collide by concatenation.
func (t Tenant) Key() string {
	return strings.Join([]string{t.Surface, t.Guild, t.Channel, t.Author}, "|")
}

// Subject is the transport's handle on the message that carried an ask.
// Nothing in this package or the coalescer inspects it beyond its identity.
type Subject interface {
	// ID names the message inside its transport, so a done mark and a dedupe
	// can each say exactly which comment they covered.
	ID() string
}

// Ask is one member comment admitted for work.
type Ask struct {
	// Seq is assigned at ingress and is monotonic across every surface, so a
	// flush can order by arrival without consulting a clock that may skew.
	Seq int64
	// Tenant shards the work. Locus is the finer target inside it: a thread on
	// Discord, a page on the site.
	Tenant Tenant
	Locus  string
	Text   string
	// At is ingress time, which is what the age cap and the window measure
	// from. The transport's own timestamp is not trusted for either.
	At      time.Time
	Subject Subject
}

// Clock is the only time source this package reads, so a test drives ingress
// without sleeping.
type Clock func() time.Time

func (c Clock) now() time.Time {
	if c == nil {
		return time.Now().UTC()
	}
	return c().UTC()
}
