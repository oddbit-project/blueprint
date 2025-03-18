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
	defer mkv.m.RUnlock()
	v, ok := mkv.data[k]
	if !ok {
		return nil, nil // not found
	}
	if v.ttl >= 0 {
		if v.ttl < time.Now().Sub(v.created) {
			delete(mkv.data, k)
			return nil, nil // not found
		}
	}
	return v.data, nil
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
	expired := make([]string, 0)
	mkv.m.RLock()
	for k, v := range mkv.data {
		if v.ttl > 0 && v.ttl < now.Sub(v.created) {
			expired = append(expired, k)
		}
	}
	mkv.m.RUnlock()
	for _, id := range expired {
		_ = mkv.Delete(id)
	}
	return nil
}
