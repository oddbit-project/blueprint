package store

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/oddbit-project/blueprint/provider/kv"
)

// slowGetKV delays Get to widen the window between the existence check and the write
type slowGetKV struct {
	kv.KV
}

func (s *slowGetKV) Get(k string) ([]byte, error) {
	v, err := s.KV.Get(k)
	time.Sleep(20 * time.Millisecond)
	return v, err
}

// atomicSlowKV is a slowGetKV that also exposes the backend SetNX
type atomicSlowKV struct {
	slowGetKV
	kv.AtomicSetter
}

func TestKvStoreAddIfNotExists(t *testing.T) {
	tests := []struct {
		name    string
		backend func() kv.KV
	}{
		{"atomic backend", func() kv.KV { return kv.NewMemoryKV() }},
		{"non-atomic backend", func() kv.KV { return &slowGetKV{KV: kv.NewMemoryKV()} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewKvStore(tt.backend(), time.Hour)
			assert.True(t, s.AddIfNotExists("nonce"))
			assert.False(t, s.AddIfNotExists("nonce"))
			assert.True(t, s.AddIfNotExists("other"))
		})
	}
}

func TestKvStoreAddIfNotExistsAtomic(t *testing.T) {
	mem := kv.NewMemoryKV()
	backend := &atomicSlowKV{
		slowGetKV:    slowGetKV{KV: mem},
		AtomicSetter: mem.(kv.AtomicSetter),
	}
	s := NewKvStore(backend, time.Hour)

	var wg sync.WaitGroup
	var success atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.AddIfNotExists("same-nonce") {
				success.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), success.Load(), "only one concurrent AddIfNotExists may succeed")
}
