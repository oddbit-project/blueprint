package session

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestGobMarshaller(t *testing.T) {
	marshaller := NewGobMarshaller()
	testMarshaller(t, marshaller, "GobMarshaller")
}

func TestJSONMarshaller(t *testing.T) {
	marshaller := NewJSONMarshaller()
	testMarshaller(t, marshaller, "JSONMarshaller")
}

func testMarshaller(t *testing.T, marshaller Marshaller, name string) {
	t.Run(name+"_BasicMarshalUnmarshal", func(t *testing.T) {
		// Create test session data
		session := &SessionData{
			ID:           "test-session-123",
			Values:       make(map[string]any),
			LastAccessed: time.Now(),
			Created:      time.Now().Add(-1 * time.Hour),
		}

		// Add various types of data
		session.Values["string"] = "test value"
		session.Values["int"] = 42
		session.Values["float"] = 3.14
		session.Values["bool"] = true
		session.Values["slice"] = []string{"one", "two", "three"}
		session.Values["map"] = map[string]int{"a": 1, "b": 2}

		// Marshal the session
		data, err := marshaller.MarshalSession(session)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		// Unmarshal the session
		unmarshalled, err := marshaller.UnmarshalSession(data)
		require.NoError(t, err)
		require.NotNil(t, unmarshalled)

		// Verify ID
		assert.Equal(t, session.ID, unmarshalled.ID)

		// Verify values
		assert.Equal(t, session.Values["string"], unmarshalled.Values["string"])
		assert.Equal(t, session.Values["bool"], unmarshalled.Values["bool"])

		// For JSON marshaller, numbers are unmarshalled as float64
		if name == "JSONMarshaller" {
			assert.Equal(t, float64(42), unmarshalled.Values["int"])
			assert.Equal(t, 3.14, unmarshalled.Values["float"])
		} else {
			assert.Equal(t, session.Values["int"], unmarshalled.Values["int"])
			assert.Equal(t, session.Values["float"], unmarshalled.Values["float"])
		}

		// Verify time fields are close (accounting for serialization precision)
		assert.WithinDuration(t, session.LastAccessed, unmarshalled.LastAccessed, time.Millisecond)
		assert.WithinDuration(t, session.Created, unmarshalled.Created, time.Millisecond)
	})

	t.Run(name+"_EmptySession", func(t *testing.T) {
		// Test with empty session
		session := &SessionData{
			ID:           "empty-session",
			Values:       make(map[string]any),
			LastAccessed: time.Now(),
			Created:      time.Now(),
		}

		data, err := marshaller.MarshalSession(session)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		unmarshalled, err := marshaller.UnmarshalSession(data)
		require.NoError(t, err)
		require.NotNil(t, unmarshalled)

		assert.Equal(t, session.ID, unmarshalled.ID)
		assert.Empty(t, unmarshalled.Values)
	})

	t.Run(name+"_NilValues", func(t *testing.T) {
		// Test with nil values in map
		session := &SessionData{
			ID:           "nil-values-session",
			Values:       make(map[string]any),
			LastAccessed: time.Now(),
			Created:      time.Now(),
		}
		session.Values["nil"] = nil
		session.Values["valid"] = "not nil"

		data, err := marshaller.MarshalSession(session)
		require.NoError(t, err)

		unmarshalled, err := marshaller.UnmarshalSession(data)
		require.NoError(t, err)

		assert.Equal(t, session.Values["nil"], unmarshalled.Values["nil"])
		assert.Equal(t, session.Values["valid"], unmarshalled.Values["valid"])
	})

	t.Run(name+"_ComplexNestedData", func(t *testing.T) {
		// Test with complex nested structures
		session := &SessionData{
			ID:     "complex-session",
			Values: make(map[string]any),
		}

		// Add nested data structure
		session.Values["nested"] = map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": "deep value",
				"array":  []int{1, 2, 3},
			},
			"another": "value",
		}

		data, err := marshaller.MarshalSession(session)
		require.NoError(t, err)

		unmarshalled, err := marshaller.UnmarshalSession(data)
		require.NoError(t, err)

		// Verify nested structure
		nested, ok := unmarshalled.Values["nested"].(map[string]interface{})
		require.True(t, ok)

		level1, ok := nested["level1"].(map[string]interface{})
		require.True(t, ok)

		assert.Equal(t, "deep value", level1["level2"])
		assert.Equal(t, "value", nested["another"])
	})

	t.Run(name+"_InvalidData", func(t *testing.T) {
		// Test unmarshalling invalid data
		invalidData := []byte("invalid data that cannot be unmarshalled")

		unmarshalled, err := marshaller.UnmarshalSession(invalidData)
		assert.Error(t, err)
		assert.Nil(t, unmarshalled)
	})

	t.Run(name+"_EmptyData", func(t *testing.T) {
		// Test unmarshalling empty data
		emptyData := []byte{}

		unmarshalled, err := marshaller.UnmarshalSession(emptyData)
		assert.Error(t, err)
		assert.Nil(t, unmarshalled)
	})
}

func TestStoreWithJSONMarshaller(t *testing.T) {
	// Test that Store can use JSON marshaller
	config := NewConfig()
	store, err := NewStore(config, nil, nil)
	require.NoError(t, err)

	// Switch to JSON marshaller
	store.WithMarshaller(NewJSONMarshaller())

	// Generate a session
	session, id := store.Generate()
	session.Set("test", "value")
	session.Set("number", 123)

	// Save the session
	err = store.Set(id, session)
	require.NoError(t, err)

	// Retrieve the session
	retrieved, err := store.Get(id)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify data
	assert.Equal(t, "value", retrieved.Values["test"])
	// JSON unmarshals numbers as float64
	assert.Equal(t, float64(123), retrieved.Values["number"])
}

func BenchmarkMarshallers(b *testing.B) {
	// Create test session with typical data
	session := &SessionData{
		ID:           "bench-session",
		Values:       make(map[string]any),
		LastAccessed: time.Now(),
		Created:      time.Now(),
	}
	session.Values["user_id"] = "12345"
	session.Values["username"] = "testuser"
	session.Values["roles"] = []string{"admin", "user"}
	session.Values["preferences"] = map[string]string{
		"theme": "dark",
		"lang":  "en",
	}

	b.Run("GobMarshaller", func(b *testing.B) {
		marshaller := NewGobMarshaller()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			data, _ := marshaller.MarshalSession(session)
			marshaller.UnmarshalSession(data)
		}
	})

	b.Run("JSONMarshaller", func(b *testing.B) {
		marshaller := NewJSONMarshaller()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			data, _ := marshaller.MarshalSession(session)
			marshaller.UnmarshalSession(data)
		}
	})
}
