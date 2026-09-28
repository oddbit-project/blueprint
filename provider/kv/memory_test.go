package kv

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryKVNoExpiry(t *testing.T) {
	tests := []struct {
		name string
		set  func(k KV) error
	}{
		{"Set", func(k KV) error { return k.Set("key", []byte("value")) }},
		{"SetTTL zero", func(k KV) error { return k.SetTTL("key", []byte("value"), 0) }},
		{"SetTTL negative", func(k KV) error { return k.SetTTL("key", []byte("value"), -time.Second) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryKV()
			require.NoError(t, tt.set(store))

			v, err := store.Get("key")
			require.NoError(t, err)
			assert.Equal(t, []byte("value"), v)

			require.NoError(t, store.Prune())
			v, err = store.Get("key")
			require.NoError(t, err)
			assert.Equal(t, []byte("value"), v)
		})
	}
}

func TestMemoryKVExpiry(t *testing.T) {
	store := NewMemoryKV()
	require.NoError(t, store.SetTTL("short", []byte("a"), time.Millisecond))
	require.NoError(t, store.SetTTL("long", []byte("b"), time.Hour))
	time.Sleep(5 * time.Millisecond)

	v, err := store.Get("short")
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = store.Get("long")
	require.NoError(t, err)
	assert.Equal(t, []byte("b"), v)
}

func TestMemoryKVPrune(t *testing.T) {
	store := NewMemoryKV()
	require.NoError(t, store.SetTTL("short", []byte("a"), time.Millisecond))
	require.NoError(t, store.SetTTL("long", []byte("b"), time.Hour))
	time.Sleep(5 * time.Millisecond)

	require.NoError(t, store.Prune())
	mkv := store.(*memkv)
	assert.NotContains(t, mkv.data, "short")
	assert.Contains(t, mkv.data, "long")
}

// TestMemoryKVConcurrentGetExpired must be run with -race: Get deletes expired keys
func TestMemoryKVConcurrentGetExpired(t *testing.T) {
	store := NewMemoryKV()
	const keys = 100
	for i := 0; i < keys; i++ {
		require.NoError(t, store.SetTTL(fmt.Sprintf("k%d", i), []byte("v"), time.Nanosecond))
	}
	time.Sleep(time.Millisecond)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < keys; i++ {
				v, err := store.Get(fmt.Sprintf("k%d", i))
				assert.NoError(t, err)
				assert.Nil(t, v)
			}
		}()
	}
	wg.Wait()
}

// TestMemoryKVPruneKeepsResetKeys checks Prune never removes a key re-set with a fresh TTL while pruning
func TestMemoryKVPruneKeepsResetKeys(t *testing.T) {
	const keys = 20000
	for round := 0; round < 20; round++ {
		store := NewMemoryKV()
		for i := 0; i < keys; i++ {
			require.NoError(t, store.SetTTL(fmt.Sprintf("k%d", i), []byte("old"), time.Nanosecond))
		}
		time.Sleep(time.Millisecond)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			assert.NoError(t, store.Prune())
		}()
		go func() {
			defer wg.Done()
			for i := keys - 1; i >= 0; i-- {
				assert.NoError(t, store.SetTTL(fmt.Sprintf("k%d", i), []byte("new"), time.Hour))
			}
		}()
		wg.Wait()

		missing := 0
		for i := 0; i < keys; i++ {
			v, err := store.Get(fmt.Sprintf("k%d", i))
			require.NoError(t, err)
			if v == nil {
				missing++
			}
		}
		require.Zero(t, missing, "round %d: Prune deleted re-set keys", round)
	}
}

func TestMemoryKVSetNX(t *testing.T) {
	store := NewMemoryKV()
	setter, ok := store.(AtomicSetter)
	require.True(t, ok, "memory KV must implement AtomicSetter")

	tests := []struct {
		name  string
		setup func()
		key   string
		ttl   time.Duration
		want  bool
	}{
		{"new key", func() {}, "a", time.Hour, true},
		{"existing key", func() {}, "a", time.Hour, false},
		{"existing key without expiry", func() { _ = store.Set("b", []byte("x")) }, "b", time.Hour, false},
		{"expired key", func() {
			_ = store.SetTTL("c", []byte("x"), time.Nanosecond)
			time.Sleep(time.Millisecond)
		}, "c", time.Hour, true},
		{"new key no expiry", func() {}, "d", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			ok, err := setter.SetNX(tt.key, []byte("1"), tt.ttl)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ok)
		})
	}
}

func TestMemoryKVSetNXConcurrent(t *testing.T) {
	setter := NewMemoryKV().(AtomicSetter)
	var wg sync.WaitGroup
	var mx sync.Mutex
	success := 0
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := setter.SetNX("nonce", []byte("1"), time.Hour)
			assert.NoError(t, err)
			if ok {
				mx.Lock()
				success++
				mx.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, success)
}
