package collections

import (
	"fmt"
	"sync"
)

type Map[K comparable, V any] struct {
	data map[K]V
	sync.RWMutex
}

func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		data: make(map[K]V),
	}
}

func (m *Map[K, V]) Contains(key K) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.data[key]
	return ok
}

func (m *Map[K, V]) Add(key K, value V) {
	m.Lock()
	defer m.Unlock()
	m.data[key] = value
}

func (m *Map[K, V]) Get(key K) (V, error) {
	m.RLock()
	defer m.RUnlock()
	result, ok := m.data[key]
	if ok {
		return result, nil
	}
	var zero V
	return zero, fmt.Errorf("cannot find item with key %v in map", key)
}

func (m *Map[K, V]) MustGet(key K) V {
	result, err := m.Get(key)
	if err != nil {
		var zero V
		return zero
	}
	return result
}

func (m *Map[K, V]) GetKeys() []K {
	m.RLock()
	defer m.RUnlock()
	keys := make([]K, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys
}

func (m *Map[K, V]) Delete(key K) {
	m.Lock()
	defer m.Unlock()
	delete(m.data, key)
}

func (m *Map[K, V]) Purge() {
	m.Lock()
	defer m.Unlock()
	m.data = make(map[K]V)
}

func (m *Map[K, V]) Len() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.data)
}

// Convenience type aliases for common use cases
type StringListMap = Map[string, []string]
type StringMap = Map[string, string]
type IntMap = Map[int, any]

// Convenience constructors for common use cases
func NewStringListMap() *StringListMap {
	return NewMap[string, []string]()
}

func NewStringMap() *StringMap {
	return NewMap[string, string]()
}

func NewIntMap() *IntMap {
	return NewMap[int, any]()
}
