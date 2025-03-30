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
	lock sync.RWMutex
}

type decayMapEntry[V any] struct {
	Value  V
	expiry time.Time
}

// New creates a new DecayMap of key type K and value type V.
//
// Key types must be comparable to work with maps.
func New[K comparable, V any]() *Impl[K, V] {
	return &Impl[K, V]{
		data: make(map[K]decayMapEntry[V]),
	}
}

// expire forcibly expires a key by setting its time-to-live one second in the past.
func (m *Impl[K, V]) expire(key K) bool {
	m.lock.RLock()
	val, ok := m.data[key]
	m.lock.RUnlock()

	if !ok {
		return false
	}

	m.lock.Lock()
	val.expiry = time.Now().Add(-1 * time.Second)
	m.data[key] = val
	m.lock.Unlock()

	return true
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
		m.lock.Lock()
		// Since previously reading m.data[key], the value may have been updated.
		// Delete the entry only if the expiry time is still the same.
		if m.data[key].expiry.Equal(value.expiry) {
			delete(m.data, key)
		}
		m.lock.Unlock()

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
