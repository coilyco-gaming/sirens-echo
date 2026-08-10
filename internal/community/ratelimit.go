package community

import (
	"sync"
	"time"
)

// Admission bounds what a deployment can spend on untrusted summons.
// See docs/sirens-echo-admission.md for the policy and its reasoning.

// admissionOutcome is the closed set of admission results. It doubles as a
// metric label, so it never carries member-supplied text.
type admissionOutcome string

const (
	admissionAccepted admissionOutcome = "accepted"
	admissionUser     admissionOutcome = "denied_user"
	admissionContext  admissionOutcome = "denied_context"
	admissionGlobal   admissionOutcome = "denied_global"
	admissionQueue    admissionOutcome = "denied_queue"
)

// denied reports whether the outcome rejected the summon.
func (o admissionOutcome) denied() bool { return o != admissionAccepted }

// RateLimit is one token bucket policy. Burst is the depth of the bucket and
// Every is the interval at which one token returns.
type RateLimit struct {
	Burst int
	Every time.Duration
}

// enabled reports whether the policy constrains anything. A zero policy is an
// explicit opt-out rather than a bucket that denies everything.
func (r RateLimit) enabled() bool { return r.Burst > 0 && r.Every > 0 }

// RateLimitPolicy is the full admission policy for one deployment.
type RateLimitPolicy struct {
	// PerUser bounds one Discord account or one authenticated HTTP caller.
	PerUser RateLimit
	// PerContext bounds one guild, one DM, or the HTTP transport, so one busy
	// guild cannot consume the deployment's whole budget.
	PerContext RateLimit
	// Global bounds the process across every context it serves.
	Global RateLimit
	// MaxPending bounds turns waiting for the execution slot. Turns beyond it
	// are shed, because a stale queued turn still costs a completion.
	MaxPending int
	// NotifyEvery bounds how often one key learns it was limited. A reply per
	// denial would be its own spam vector.
	NotifyEvery time.Duration
}

// bucket is one key's token state. Tokens are tracked as a float so a slow
// refill interval still accrues partial credit between summons.
type bucket struct {
	tokens     float64
	lastRefill time.Time
	lastNotify time.Time
}

// rateLimiter applies a RateLimitPolicy across a capacity-bounded LRU, so
// rotating identities cannot grow process memory.
type rateLimiter struct {
	mu       sync.Mutex
	policy   RateLimitPolicy
	capacity int
	buckets  map[string]*bucket
	order    []string
	pending  int
	now      func() time.Time
}

const defaultRateLimiterCapacity = 4096

func newRateLimiter(policy RateLimitPolicy, capacity int) *rateLimiter {
	if capacity <= 0 {
		capacity = defaultRateLimiterCapacity
	}
	return &rateLimiter{
		policy:   policy,
		capacity: capacity,
		buckets:  make(map[string]*bucket, capacity),
		order:    make([]string, 0, capacity),
		now:      time.Now,
	}
}

// admissionRequest identifies the principal and context behind one summon.
type admissionRequest struct {
	// UserKey identifies the requesting account. Empty disables per-user
	// limiting for this summon.
	UserKey string
	// ContextKey identifies the guild, DM, or transport the summon arrived on.
	ContextKey string
	// Queued marks a request that occupies the execution slot, so it counts
	// against MaxPending and must be released. Cheap gates leave it false.
	Queued bool
}

// admissionDecision is the limiter's verdict plus whether this caller should be
// told about a denial.
type admissionDecision struct {
	Outcome admissionOutcome
	// Notify is true only on the first denial for this key inside the policy's
	// NotifyEvery window.
	Notify bool
	// RetryAfter is how long until the exhausted bucket yields one token.
	RetryAfter time.Duration
}

// Admit consumes one token from every enabled bucket the request touches.
// Every bucket is checked before any is charged.
func (l *rateLimiter) Admit(request admissionRequest) admissionDecision {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	type candidate struct {
		key     string
		limit   RateLimit
		outcome admissionOutcome
	}
	candidates := make([]candidate, 0, 3)
	if request.UserKey != "" {
		candidates = append(candidates, candidate{"user:" + request.UserKey, l.policy.PerUser, admissionUser})
	}
	if request.ContextKey != "" {
		candidates = append(candidates, candidate{"context:" + request.ContextKey, l.policy.PerContext, admissionContext})
	}
	candidates = append(candidates, candidate{"global", l.policy.Global, admissionGlobal})

	charge := make([]*bucket, 0, len(candidates))
	for _, current := range candidates {
		if !current.limit.enabled() {
			continue
		}
		state := l.bucketFor(current.key, current.limit, now)
		if state.tokens < 1 {
			return l.denyLocked(state, current.limit, current.outcome, now)
		}
		charge = append(charge, state)
	}

	if request.Queued && l.policy.MaxPending > 0 && l.pending >= l.policy.MaxPending {
		// Keyed on the context so one guild's flood does not mute another
		// guild's cooldown notice.
		state := l.bucketFor("queue:"+request.ContextKey, RateLimit{Burst: 1, Every: time.Second}, now)
		return l.denyLocked(state, RateLimit{}, admissionQueue, now)
	}

	for _, state := range charge {
		state.tokens--
	}
	if request.Queued {
		l.pending++
	}
	return admissionDecision{Outcome: admissionAccepted}
}

// Release returns one pending slot. Every accepted Admit owns exactly one.
func (l *rateLimiter) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pending > 0 {
		l.pending--
	}
}

// denyLocked builds a denial and decides whether this key is due a notice.
// The caller must hold l.mu.
func (l *rateLimiter) denyLocked(
	state *bucket,
	limit RateLimit,
	outcome admissionOutcome,
	now time.Time,
) admissionDecision {
	decision := admissionDecision{Outcome: outcome}
	if limit.enabled() {
		decision.RetryAfter = time.Duration((1 - state.tokens) * float64(limit.Every))
	}
	if l.policy.NotifyEvery <= 0 ||
		state.lastNotify.IsZero() ||
		now.Sub(state.lastNotify) >= l.policy.NotifyEvery {
		state.lastNotify = now
		decision.Notify = true
	}
	return decision
}

// bucketFor returns one key's refilled bucket, evicting the oldest key at
// capacity. The caller must hold l.mu.
func (l *rateLimiter) bucketFor(key string, limit RateLimit, now time.Time) *bucket {
	state, exists := l.buckets[key]
	if !exists {
		state = &bucket{tokens: float64(limit.Burst), lastRefill: now}
		l.buckets[key] = state
		l.order = append(l.order, key)
		if len(l.order) > l.capacity {
			oldest := l.order[0]
			l.order = l.order[1:]
			delete(l.buckets, oldest)
		}
		return state
	}
	if limit.enabled() && now.After(state.lastRefill) {
		refilled := now.Sub(state.lastRefill).Seconds() / limit.Every.Seconds()
		state.tokens += refilled
		if state.tokens > float64(limit.Burst) {
			state.tokens = float64(limit.Burst)
		}
		state.lastRefill = now
	}
	return state
}
