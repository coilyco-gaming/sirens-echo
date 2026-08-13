package community

import (
	"fmt"
	"testing"
	"time"
)

// testLimiter builds a limiter with a clock the test advances by hand, so the
// suite never sleeps and refill behavior is exact rather than approximate.
func testLimiter(policy RateLimitPolicy, capacity int) (*rateLimiter, func(time.Duration)) {
	limiter := newRateLimiter(policy, capacity)
	clock := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return clock }
	return limiter, func(step time.Duration) { clock = clock.Add(step) }
}

func TestRateLimiterSpendsBurstThenDenies(t *testing.T) {
	limiter, advance := testLimiter(RateLimitPolicy{
		PerUser: RateLimit{Burst: 3, Every: 30 * time.Second},
	}, 16)
	request := admissionRequest{UserKey: "member", ContextKey: "guild:1"}

	for attempt := 1; attempt <= 3; attempt++ {
		if got := limiter.Admit(request).Outcome; got != admissionAccepted {
			t.Fatalf("attempt %d: outcome = %q, want accepted", attempt, got)
		}
	}
	decision := limiter.Admit(request)
	if decision.Outcome != admissionUser {
		t.Fatalf("outcome = %q, want %q", decision.Outcome, admissionUser)
	}
	if decision.RetryAfter <= 0 || decision.RetryAfter > 30*time.Second {
		t.Fatalf("RetryAfter = %s, want a value inside the refill interval", decision.RetryAfter)
	}

	// One interval returns exactly one token, not the whole burst.
	advance(30 * time.Second)
	if got := limiter.Admit(request).Outcome; got != admissionAccepted {
		t.Fatalf("after refill: outcome = %q, want accepted", got)
	}
	if got := limiter.Admit(request).Outcome; got != admissionUser {
		t.Fatalf("second summon after one refill: outcome = %q, want denied", got)
	}
}

func TestRateLimiterKeepsGuildsIndependent(t *testing.T) {
	limiter, _ := testLimiter(RateLimitPolicy{
		PerContext: RateLimit{Burst: 2, Every: time.Minute},
	}, 16)
	noisy := admissionRequest{UserKey: "a", ContextKey: "guild:noisy"}
	quiet := admissionRequest{UserKey: "b", ContextKey: "guild:quiet"}

	for attempt := 0; attempt < 2; attempt++ {
		if got := limiter.Admit(noisy).Outcome; got != admissionAccepted {
			t.Fatalf("noisy guild attempt %d: outcome = %q", attempt, got)
		}
	}
	if got := limiter.Admit(noisy).Outcome; got != admissionContext {
		t.Fatalf("exhausted guild: outcome = %q, want %q", got, admissionContext)
	}
	// A guild that floods must not spend another guild's budget.
	if got := limiter.Admit(quiet).Outcome; got != admissionAccepted {
		t.Fatalf("quiet guild: outcome = %q, want accepted", got)
	}
}

// A request denied by a later tier must not consume the earlier tiers it
// already passed.
func TestRateLimiterDoesNotChargeWhenALaterTierDenies(t *testing.T) {
	limiter, _ := testLimiter(RateLimitPolicy{
		PerUser: RateLimit{Burst: 5, Every: time.Minute},
		Global:  RateLimit{Burst: 1, Every: time.Minute},
	}, 16)
	flooder := admissionRequest{UserKey: "flooder", ContextKey: "guild:1"}
	other := admissionRequest{UserKey: "other", ContextKey: "guild:1"}

	if got := limiter.Admit(flooder).Outcome; got != admissionAccepted {
		t.Fatalf("first summon: outcome = %q", got)
	}
	for attempt := 0; attempt < 3; attempt++ {
		if got := limiter.Admit(flooder).Outcome; got != admissionGlobal {
			t.Fatalf("attempt %d: outcome = %q, want %q", attempt, got, admissionGlobal)
		}
	}
	// The flooder spent one user token, not four.
	limiter.policy.Global = RateLimit{Burst: 10, Every: time.Minute}
	limiter.buckets["global"].tokens = 10
	for attempt := 0; attempt < 4; attempt++ {
		if got := limiter.Admit(flooder).Outcome; got != admissionAccepted {
			t.Fatalf("post-recovery attempt %d: outcome = %q, want accepted", attempt, got)
		}
	}
	if got := limiter.Admit(other).Outcome; got != admissionAccepted {
		t.Fatalf("unrelated member: outcome = %q, want accepted", got)
	}
}

func TestRateLimiterShedsBeyondMaxPending(t *testing.T) {
	limiter, _ := testLimiter(RateLimitPolicy{MaxPending: 2}, 16)
	request := admissionRequest{UserKey: "member", ContextKey: "guild:1", Queued: true}

	for attempt := 0; attempt < 2; attempt++ {
		if got := limiter.Admit(request).Outcome; got != admissionAccepted {
			t.Fatalf("attempt %d: outcome = %q", attempt, got)
		}
	}
	if got := limiter.Admit(request).Outcome; got != admissionQueue {
		t.Fatalf("full queue: outcome = %q, want %q", got, admissionQueue)
	}
	limiter.Release()
	if got := limiter.Admit(request).Outcome; got != admissionAccepted {
		t.Fatalf("after release: outcome = %q, want accepted", got)
	}

	// Unqueued admissions gate cheap work and must not occupy the queue.
	limiter.Release()
	limiter.Release()
	lookup := admissionRequest{ContextKey: "guild:1"}
	for attempt := 0; attempt < 5; attempt++ {
		if got := limiter.Admit(lookup).Outcome; got != admissionAccepted {
			t.Fatalf("lookup %d: outcome = %q, want accepted", attempt, got)
		}
	}
}

func TestRateLimiterNotifiesOncePerWindow(t *testing.T) {
	limiter, advance := testLimiter(RateLimitPolicy{
		PerUser:     RateLimit{Burst: 1, Every: time.Hour},
		NotifyEvery: 5 * time.Minute,
	}, 16)
	request := admissionRequest{UserKey: "member", ContextKey: "guild:1"}
	limiter.Admit(request)

	first := limiter.Admit(request)
	if !first.Notify {
		t.Fatal("the first denial in a window must notify")
	}
	for attempt := 0; attempt < 10; attempt++ {
		if limiter.Admit(request).Notify {
			t.Fatal("a flood must not produce a reply per denied summon")
		}
	}
	advance(5 * time.Minute)
	if !limiter.Admit(request).Notify {
		t.Fatal("a denial after the notify window must notify again")
	}
}

func TestRateLimiterBoundsTrackedKeys(t *testing.T) {
	const capacity = 8
	limiter, _ := testLimiter(RateLimitPolicy{
		PerUser: RateLimit{Burst: 1, Every: time.Hour},
	}, capacity)

	// Rotating identities is the cheapest way to attack a limiter that keeps
	// unbounded per-key state.
	for index := 0; index < capacity*20; index++ {
		limiter.Admit(admissionRequest{
			UserKey:    string(rune('a'+index%26)) + string(rune('a'+index/26)),
			ContextKey: "guild:1",
		})
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.buckets) > capacity {
		t.Fatalf("tracked keys = %d, want at most %d", len(limiter.buckets), capacity)
	}
	if len(limiter.order) != len(limiter.buckets) {
		t.Fatalf("order = %d, buckets = %d, want equal", len(limiter.order), len(limiter.buckets))
	}
}

func TestRateLimiterZeroPolicyAdmitsEverything(t *testing.T) {
	limiter, _ := testLimiter(RateLimitPolicy{}, 16)
	for attempt := 0; attempt < 50; attempt++ {
		if got := limiter.Admit(admissionRequest{
			UserKey:    "member",
			ContextKey: "guild:1",
			Queued:     true,
		}).Outcome; got != admissionAccepted {
			t.Fatalf("attempt %d: outcome = %q, want accepted", attempt, got)
		}
	}
}

// A queue shed is the denial class that dominates under burst, which is exactly
// when a client needs backoff guidance. It must carry Retry-After like the rest.
func TestQueueShedCarriesRetryAfter(t *testing.T) {
	t.Parallel()
	limiter := newRateLimiter(RateLimitPolicy{
		PerUser:    RateLimit{Burst: 100, Every: time.Second},
		PerContext: RateLimit{Burst: 100, Every: time.Second},
		Global:     RateLimit{Burst: 100, Every: time.Second},
		MaxPending: 1,
	}, 16)

	first := limiter.Admit(admissionRequest{UserKey: "u1", ContextKey: "c1", Queued: true})
	if first.Outcome.denied() {
		t.Fatalf("first request denied: %v", first.Outcome)
	}
	shed := limiter.Admit(admissionRequest{UserKey: "u2", ContextKey: "c1", Queued: true})
	if shed.Outcome != admissionQueue {
		t.Fatalf("outcome = %v, want a queue shed", shed.Outcome)
	}
	if shed.RetryAfter <= 0 {
		t.Fatal("queue shed carried no Retry-After, contradicting the documented contract")
	}
	if shed.RetryAfter > time.Second {
		t.Fatalf("Retry-After = %v, over the one second shed window", shed.RetryAfter)
	}
}

// The reported bypass. A caller-asserted header can mint unlimited user keys,
// and insertion-order eviction dropped the busiest, earliest bucket of all.
func TestRotatingKeysCannotResetTheGlobalBudget(t *testing.T) {
	t.Parallel()
	limiter := newRateLimiter(RateLimitPolicy{
		PerUser: RateLimit{Burst: 100, Every: time.Hour},
		Global:  RateLimit{Burst: 2, Every: time.Hour},
	}, 4)

	accepted := 0
	for attempt := 0; attempt < 40; attempt++ {
		decision := limiter.Admit(admissionRequest{
			UserKey:    fmt.Sprintf("http:rotating-%d", attempt),
			ContextKey: transportHTTP,
		})
		if decision.Outcome == admissionAccepted {
			accepted++
		}
	}
	if accepted > 2 {
		t.Fatalf("%d admissions against a global burst of 2; the global bucket was reset", accepted)
	}
}

// The capacity bound still holds, so key churn cannot grow the table.
func TestKeyChurnStaysBounded(t *testing.T) {
	t.Parallel()
	limiter := newRateLimiter(RateLimitPolicy{
		PerUser: RateLimit{Burst: 100, Every: time.Hour},
		Global:  RateLimit{Burst: 1000, Every: time.Hour},
	}, 4)
	for attempt := 0; attempt < 40; attempt++ {
		limiter.Admit(admissionRequest{UserKey: fmt.Sprintf("u%d", attempt), ContextKey: "c"})
	}
	// The global and context buckets sit outside or inside the bound; what must
	// not happen is unbounded growth.
	if len(limiter.buckets) > limiter.capacity+2 {
		t.Fatalf("bucket table grew to %d against a capacity of %d", len(limiter.buckets), limiter.capacity)
	}
}

// Eviction is by recency, so a bucket in active use survives churn around it.
func TestAnActiveBucketSurvivesChurn(t *testing.T) {
	t.Parallel()
	limiter := newRateLimiter(RateLimitPolicy{
		PerUser: RateLimit{Burst: 2, Every: time.Hour},
		Global:  RateLimit{Burst: 1000, Every: time.Hour},
	}, 4)

	steady := admissionRequest{UserKey: "http:steady", ContextKey: transportHTTP}
	if got := limiter.Admit(steady).Outcome; got != admissionAccepted {
		t.Fatalf("first steady admission = %v", got)
	}
	// Churn past the capacity while the steady key keeps being used.
	for attempt := 0; attempt < 20; attempt++ {
		limiter.Admit(admissionRequest{
			UserKey:    fmt.Sprintf("http:churn-%d", attempt),
			ContextKey: transportHTTP,
		})
		limiter.Admit(steady)
	}
	// Its burst of 2 is long spent, so a surviving bucket denies.
	if got := limiter.Admit(steady).Outcome; got == admissionAccepted {
		t.Fatal("the steady bucket was evicted and came back at full burst")
	}
}
