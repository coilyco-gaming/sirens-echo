package coalesce

import "sync"

// tenantLocks gives one writer per tenant without serializing the pool. A
// global lock would undo the three workers the arithmetic needs.
type tenantLocks struct {
	mu   sync.Mutex
	held map[string]*tenantLock
}

// tenantLock counts its waiters, so the map holds a lock only while a tenant
// has one. Without the count a long-lived deployment leaks an entry per member.
type tenantLock struct {
	mu   sync.Mutex
	refs int
}

func newTenantLocks() *tenantLocks {
	return &tenantLocks{held: make(map[string]*tenantLock)}
}

// Lock blocks until this tenant has no other writer and returns its release.
func (t *tenantLocks) Lock(key string) func() {
	t.mu.Lock()
	entry, ok := t.held[key]
	if !ok {
		entry = &tenantLock{}
		t.held[key] = entry
	}
	entry.refs++
	t.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		t.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(t.held, key)
		}
		t.mu.Unlock()
	}
}

// tracked reports how many tenants hold a lock, which is what proves the map
// does not grow with the membership.
func (t *tenantLocks) tracked() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.held)
}
