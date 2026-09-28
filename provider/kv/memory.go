package kv

import (
	"sync"
	"time"
)

type KV interface {
	SetTTL(k string, v []byte, ttl time.Duration) error
	Set(k string, v []byte) error
	Get(k string) ([]byte, error)
	Delete(k string) error
	Prune() error
}

// AtomicSetter is implemented by KV backends that support an atomic set-if-not-exists operation
type AtomicSetter interface {
	// SetNX sets a key value with ttl only if the key does not exist; returns true if the key was set
	SetNX(k string, v []byte, ttl time.Duration) (bool, error)
}

type record struct {
	data    []byte
	created time.Time
	ttl     time.Duration
}

type memkv struct {
	data map[string]*record
	m    sync.RWMutex
}

func NewMemoryKV() KV {
	return &memkv{
		data: make(map[string]*record),
		m:    sync.RWMutex{},
	}
}

// Set sets a key value
func (mkv *memkv) Set(k string, v []byte) error {
	mkv.m.Lock()
	defer mkv.m.Unlock()
	mkv.data[k] = &record{
		data: v,
		ttl:  0,
	}
	return nil
}

// SetTTL sets a key value with ttl
func (mkv *memkv) SetTTL(k string, v []byte, ttl time.Duration) error {
	mkv.m.Lock()
	defer mkv.m.Unlock()
	mkv.data[k] = &record{
		data:    v,
		created: time.Now(),
		ttl:     ttl,
	}
	return nil
}

// Get fetches a value
func (mkv *memkv) Get(k string) ([]byte, error) {
	mkv.m.RLock()
	v, ok := mkv.data[k]
	mkv.m.RUnlock()
	if !ok {
		return nil, nil // not found
	}
	if v.expired(time.Now()) {
		mkv.m.Lock()
		if mkv.data[k] == v {
			delete(mkv.data, k)
		}
		mkv.m.Unlock()
		return nil, nil // not found
	}
	return v.data, nil
}

// SetNX sets a key value with ttl only if the key does not exist or is expired
func (mkv *memkv) SetNX(k string, v []byte, ttl time.Duration) (bool, error) {
	mkv.m.Lock()
	defer mkv.m.Unlock()
	if r, ok := mkv.data[k]; ok && !r.expired(time.Now()) {
		return false, nil
	}
	mkv.data[k] = &record{
		data:    v,
		created: time.Now(),
		ttl:     ttl,
	}
	return true, nil
}

// Del remove a value
func (mkv *memkv) Delete(k string) error {
	mkv.m.Lock()
	defer mkv.m.Unlock()
	delete(mkv.data, k)
	return nil
}

// Prune removes expired records
func (mkv *memkv) Prune() error {
	now := time.Now()
	mkv.m.Lock()
	defer mkv.m.Unlock()
	for k, v := range mkv.data {
		if v.expired(now) {
			delete(mkv.data, k)
		}
	}
	return nil
}

// expired returns true if the record has a positive ttl that elapsed; ttl <= 0 means no expiry
func (r *record) expired(now time.Time) bool {
	return r.ttl > 0 && now.Sub(r.created) > r.ttl
}
