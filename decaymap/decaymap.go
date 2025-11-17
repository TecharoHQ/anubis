package decaymap

import (
	"sync"
	"time"
)

func Zilch[T any]() T {
	var zero T
	return zero
}

// Impl is a lazy key->value map. It's a wrapper around a map and a mutex. If values exceed their time-to-live, they are pruned at Get time.
type Impl[K comparable, V any] struct {
	data map[K]decayMapEntry[V]

	// deleteCh receives decay-deletion requests from readers.
	deleteCh chan deleteReq[K]
	// stopCh stops the background cleanup worker.
	stopCh chan struct{}
	wg     sync.WaitGroup
	lock   sync.RWMutex
}

type decayMapEntry[V any] struct {
	Value  V
	expiry time.Time
}

// deleteReq is a request to remove a key if its expiry timestamp still matches
// the observed one. This prevents racing with concurrent Set updates.
type deleteReq[K comparable] struct {
	key    K
	expiry time.Time
}

// New creates a new DecayMap of key type K and value type V.
//
// Key types must be comparable to work with maps.
func New[K comparable, V any]() *Impl[K, V] {
	m := &Impl[K, V]{
		data:     make(map[K]decayMapEntry[V]),
		deleteCh: make(chan deleteReq[K], 1024),
		stopCh:   make(chan struct{}),
	}
	m.wg.Add(1)
	go m.cleanupWorker()
	return m
}

// expire forcibly expires a key by setting its time-to-live one second in the past.
func (m *Impl[K, V]) expire(key K) bool {
	// Use a single write lock to avoid RUnlock->Lock convoy.
	m.lock.Lock()
	defer m.lock.Unlock()
	val, ok := m.data[key]
	if !ok {
		return false
	}
	val.expiry = time.Now().Add(-1 * time.Second)
	m.data[key] = val
	return true
}

// Delete a value from the DecayMap by key.
//
// If the value does not exist, return false. Return true after
// deletion.
func (m *Impl[K, V]) Delete(key K) bool {
	// Use a single write lock to avoid RUnlock->Lock convoy.
	m.lock.Lock()
	defer m.lock.Unlock()
	_, ok := m.data[key]
	if ok {
		delete(m.data, key)
	}
	return ok
}

// Get gets a value from the DecayMap by key.
//
// If a value has expired, forcibly delete it if it was not updated.
func (m *Impl[K, V]) Get(key K) (V, bool) {
	m.lock.RLock()
	value, ok := m.data[key]
	m.lock.RUnlock()

	if !ok {
		return Zilch[V](), false
	}

	if time.Now().After(value.expiry) {
		// Defer decay deletion to the background worker to avoid convoy.
		select {
		case m.deleteCh <- deleteReq[K]{key: key, expiry: value.expiry}:
		default:
			// Channel full: drop request; a future Cleanup() or Get will retry.
		}

		return Zilch[V](), false
	}

	return value.Value, true
}

// Set sets a key value pair in the map.
func (m *Impl[K, V]) Set(key K, value V, ttl time.Duration) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.data[key] = decayMapEntry[V]{
		Value:  value,
		expiry: time.Now().Add(ttl),
	}
}

// Cleanup removes all expired entries from the DecayMap.
func (m *Impl[K, V]) Cleanup() {
	m.lock.Lock()
	defer m.lock.Unlock()

	now := time.Now()
	for key, entry := range m.data {
		if now.After(entry.expiry) {
			delete(m.data, key)
		}
	}
}

// Len returns the number of entries in the DecayMap.
func (m *Impl[K, V]) Len() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return len(m.data)
}

// Close stops the background cleanup worker. It's optional to call; maps live
// for the process lifetime in many cases. Call in tests or when you know you no
// longer need the map to avoid goroutine leaks.
func (m *Impl[K, V]) Close() {
	close(m.stopCh)
	m.wg.Wait()
}

// cleanupWorker batches decay deletions to minimize lock contention.
func (m *Impl[K, V]) cleanupWorker() {
	defer m.wg.Done()
	batch := make([]deleteReq[K], 0, 64)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		m.applyDeletes(batch)
		// reset batch without reallocating
		batch = batch[:0]
	}

	for {
		select {
		case req := <-m.deleteCh:
			batch = append(batch, req)
		case <-ticker.C:
			flush()
		case <-m.stopCh:
			// Drain any remaining requests then exit
			for {
				select {
				case req := <-m.deleteCh:
					batch = append(batch, req)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (m *Impl[K, V]) applyDeletes(batch []deleteReq[K]) {
	now := time.Now()
	m.lock.Lock()
	for _, req := range batch {
		entry, ok := m.data[req.key]
		if !ok {
			continue
		}
		// Only delete if the expiry is unchanged and already past.
		if entry.expiry.Equal(req.expiry) && now.After(entry.expiry) {
			delete(m.data, req.key)
		}
	}
	m.lock.Unlock()
}
