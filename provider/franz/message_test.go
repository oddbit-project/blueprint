package franz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRecord(t *testing.T) {
	value := []byte("test value")
	record := NewRecord(value)

	assert.Equal(t, value, record.Value)
	assert.Equal(t, int32(-1), record.Partition)
	assert.Nil(t, record.Key)
	assert.Empty(t, record.Headers)
	assert.True(t, record.Timestamp.IsZero())
}

func TestRecordBuilder(t *testing.T) {
	value := []byte("test value")
	key := []byte("test key")
	topic := "test-topic"
	partition := int32(5)
	ts := time.Now()

	record := NewRecord(value).
		WithKey(key).
		WithTopic(topic).
		WithPartition(partition).
		WithTimestamp(ts).
		WithHeader("header1", []byte("value1")).
		WithHeaders(Header{Key: "header2", Value: []byte("value2")})

	assert.Equal(t, value, record.Value)
	assert.Equal(t, key, record.Key)
	assert.Equal(t, topic, record.Topic)
	assert.Equal(t, partition, record.Partition)
	assert.Equal(t, ts, record.Timestamp)
	assert.Len(t, record.Headers, 2)
	assert.Equal(t, "header1", record.Headers[0].Key)
	assert.Equal(t, []byte("value1"), record.Headers[0].Value)
	assert.Equal(t, "header2", record.Headers[1].Key)
	assert.Equal(t, []byte("value2"), record.Headers[1].Value)
}

func TestFetchResult(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		result := &FetchResult{}

		assert.True(t, result.IsEmpty())
		assert.Equal(t, 0, result.RecordCount())
		assert.Empty(t, result.Records())
		assert.False(t, result.HasErrors())
		assert.Nil(t, result.FirstError())
	})

	t.Run("result with records", func(t *testing.T) {
		result := &FetchResult{
			Batches: []Batch{
				{
					Topic:     "topic1",
					Partition: 0,
					Records: []ConsumedRecord{
						{Topic: "topic1", Partition: 0, Offset: 0, Value: []byte("msg1")},
						{Topic: "topic1", Partition: 0, Offset: 1, Value: []byte("msg2")},
					},
				},
				{
					Topic:     "topic1",
					Partition: 1,
					Records: []ConsumedRecord{
						{Topic: "topic1", Partition: 1, Offset: 0, Value: []byte("msg3")},
					},
				},
			},
		}

		assert.False(t, result.IsEmpty())
		assert.Equal(t, 3, result.RecordCount())
		assert.Len(t, result.Records(), 3)
		assert.False(t, result.HasErrors())
	})

	t.Run("result with errors", func(t *testing.T) {
		result := &FetchResult{
			Errors: []FetchError{
				{Topic: "topic1", Partition: 0, Err: ErrClientClosed},
			},
		}

		assert.True(t, result.IsEmpty())
		assert.True(t, result.HasErrors())
		assert.Equal(t, ErrClientClosed, result.FirstError())
	})
}

func TestRecordToKgo(t *testing.T) {
	t.Run("basic record", func(t *testing.T) {
		record := NewRecord([]byte("value")).
			WithKey([]byte("key")).
			WithTopic("topic")

		kgoRecord := recordToKgo(record, "default-topic")

		assert.Equal(t, "topic", kgoRecord.Topic)
		assert.Equal(t, []byte("key"), kgoRecord.Key)
		assert.Equal(t, []byte("value"), kgoRecord.Value)
	})

	t.Run("uses default topic", func(t *testing.T) {
		record := NewRecord([]byte("value"))

		kgoRecord := recordToKgo(record, "default-topic")

		assert.Equal(t, "default-topic", kgoRecord.Topic)
	})

	t.Run("with headers", func(t *testing.T) {
		record := NewRecord([]byte("value")).
			WithHeader("h1", []byte("v1")).
			WithHeader("h2", []byte("v2"))

		kgoRecord := recordToKgo(record, "topic")

		assert.Len(t, kgoRecord.Headers, 2)
		assert.Equal(t, "h1", kgoRecord.Headers[0].Key)
		assert.Equal(t, []byte("v1"), kgoRecord.Headers[0].Value)
	})

	t.Run("with timestamp", func(t *testing.T) {
		ts := time.Now()
		record := NewRecord([]byte("value")).WithTimestamp(ts)

		kgoRecord := recordToKgo(record, "topic")

		assert.Equal(t, ts, kgoRecord.Timestamp)
	})
}
