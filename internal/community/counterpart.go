package community

import (
	"sync"
	"time"
)

// Recognition is grounded in Discord's bot flag, never inferred from prose.
// See docs/sirens-echo-compose.md.

// CounterpartKind is what the author of a message is. It is grounded, so it
// carries only what Discord asserted.
type CounterpartKind string

const (
	// CounterpartHuman is an ordinary member account.
	CounterpartHuman CounterpartKind = "human"
	// CounterpartAgent is an account Discord marks as a bot.
	CounterpartAgent CounterpartKind = "agent"
)

// exchangeLimiter bounds a run of agent-to-agent turns per channel.
type exchangeLimiter struct {
	mu      sync.Mutex
	runs    map[string]exchangeRun
	now     func() time.Time
	limit   int
	window  time.Duration
	maxKeys int
}

type exchangeRun struct {
	count int
	last  time.Time
}

func newExchangeLimiter(now func() time.Time) *exchangeLimiter {
	if now == nil {
		now = time.Now
	}
	return &exchangeLimiter{
		runs:    make(map[string]exchangeRun),
		now:     now,
		limit:   maxAgentExchange,
		window:  agentExchangeWindow,
		maxKeys: 256,
	}
}

// admit reports whether an agent-authored turn may run in this channel. A
// human turn resets the run, because a person joining ends the loop.
func (l *exchangeLimiter) admit(channelID string, kind CounterpartKind) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	moment := l.now()
	if kind != CounterpartAgent {
		delete(l.runs, channelID)
		return true
	}
	run, seen := l.runs[channelID]
	if !seen || moment.Sub(run.last) > l.window {
		run = exchangeRun{}
	}
	if run.count >= l.limit {
		// Refresh the timestamp so the bound holds while the pair keeps trying,
		// rather than expiring mid-exchange and letting it resume.
		run.last = moment
		l.runs[channelID] = run
		return false
	}
	run.count++
	run.last = moment
	l.evictLocked()
	l.runs[channelID] = run
	return true
}

// evictLocked keeps the map bounded, so a busy deployment cannot grow it
// without limit.
func (l *exchangeLimiter) evictLocked() {
	if len(l.runs) < l.maxKeys {
		return
	}
	oldestKey := ""
	var oldest time.Time
	for key, run := range l.runs {
		if oldestKey == "" || run.last.Before(oldest) {
			oldestKey, oldest = key, run.last
		}
	}
	delete(l.runs, oldestKey)
}
