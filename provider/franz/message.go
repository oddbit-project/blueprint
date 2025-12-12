package franz

import (
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Header represents a Kafka record header
type Header struct {
	Key   string
	Value []byte
}

// Record represents a Kafka record for producing
type Record struct {
	Topic     string    // Target topic (optional if default set)
	Key       []byte    // Record key
	Value     []byte    // Record value
	Headers   []Header  // Record headers
	Partition int32     // Partition (-1 for auto-assignment)
	Timestamp time.Time // Timestamp (zero for broker timestamp)
}

// NewRecord creates a record with value only
func NewRecord(value []byte) *Record {
	return &Record{
		Value:     value,
		Partition: -1,
	}
}

// WithKey adds a key to the record
func (r *Record) WithKey(key []byte) *Record {
	r.Key = key
	return r
}

// WithTopic sets the target topic
func (r *Record) WithTopic(topic string) *Record {
	r.Topic = topic
	return r
}

// WithPartition sets a specific partition
func (r *Record) WithPartition(partition int32) *Record {
	r.Partition = partition
	return r
}

// WithTimestamp sets the record timestamp
func (r *Record) WithTimestamp(ts time.Time) *Record {
	r.Timestamp = ts
	return r
}

// WithHeader adds a single header to the record
func (r *Record) WithHeader(key string, value []byte) *Record {
	r.Headers = append(r.Headers, Header{Key: key, Value: value})
	return r
}

// WithHeaders adds multiple headers to the record
func (r *Record) WithHeaders(headers ...Header) *Record {
	r.Headers = append(r.Headers, headers...)
	return r
}

// ConsumedRecord represents a received Kafka record with metadata
type ConsumedRecord struct {
	Topic       string
	Partition   int32
	Offset      int64
	Key         []byte
	Value       []byte
	Headers     []Header
	Timestamp   time.Time
	LeaderEpoch int32
}

// Batch represents a batch of consumed records from a single partition
type Batch struct {
	Records   []ConsumedRecord
	Topic     string
	Partition int32
}

// FetchResult represents the result of a poll operation
type FetchResult struct {
	Batches []Batch
	Errors  []FetchError
}

// FetchError represents an error for a specific topic/partition
type FetchError struct {
	Topic     string
	Partition int32
	Err       error
}

// IsEmpty returns true if no records were fetched
func (f *FetchResult) IsEmpty() bool {
	return len(f.Batches) == 0
}

// RecordCount returns the total number of records fetched
func (f *FetchResult) RecordCount() int {
	count := 0
	for _, b := range f.Batches {
		count += len(b.Records)
	}
	return count
}

// Records returns all records flattened into a single slice
func (f *FetchResult) Records() []ConsumedRecord {
	records := make([]ConsumedRecord, 0, f.RecordCount())
	for _, b := range f.Batches {
		records = append(records, b.Records...)
	}
	return records
}

// HasErrors returns true if any fetch errors occurred
func (f *FetchResult) HasErrors() bool {
	return len(f.Errors) > 0
}

// FirstError returns the first error, or nil if no errors
func (f *FetchResult) FirstError() error {
	if len(f.Errors) == 0 {
		return nil
	}
	return f.Errors[0].Err
}

// recordToKgo converts a Record to a kgo.Record
func recordToKgo(r *Record, defaultTopic string) *kgo.Record {
	topic := r.Topic
	if topic == "" {
		topic = defaultTopic
	}

	headers := make([]kgo.RecordHeader, len(r.Headers))
	for i, h := range r.Headers {
		headers[i] = kgo.RecordHeader{Key: h.Key, Value: h.Value}
	}

	rec := &kgo.Record{
		Topic:   topic,
		Key:     r.Key,
		Value:   r.Value,
		Headers: headers,
	}

	if r.Partition >= 0 {
		rec.Partition = r.Partition
	}

	if !r.Timestamp.IsZero() {
		rec.Timestamp = r.Timestamp
	}

	return rec
}

// kgoToConsumedRecord converts a kgo.Record to a ConsumedRecord
func kgoToConsumedRecord(r *kgo.Record) ConsumedRecord {
	headers := make([]Header, len(r.Headers))
	for i, h := range r.Headers {
		headers[i] = Header{Key: h.Key, Value: h.Value}
	}

	return ConsumedRecord{
		Topic:       r.Topic,
		Partition:   r.Partition,
		Offset:      r.Offset,
		Key:         r.Key,
		Value:       r.Value,
		Headers:     headers,
		Timestamp:   r.Timestamp,
		LeaderEpoch: r.LeaderEpoch,
	}
}

// fetchesToResult converts kgo.Fetches to a FetchResult
func fetchesToResult(fetches kgo.Fetches) *FetchResult {
	result := &FetchResult{}

	// Collect errors
	if errs := fetches.Errors(); len(errs) > 0 {
		result.Errors = make([]FetchError, len(errs))
		for i, e := range errs {
			result.Errors[i] = FetchError{
				Topic:     e.Topic,
				Partition: e.Partition,
				Err:       e.Err,
			}
		}
	}

	// Collect records by topic/partition
	fetches.EachPartition(func(ftp kgo.FetchTopicPartition) {
		if len(ftp.Records) == 0 {
			return
		}

		batch := Batch{
			Topic:     ftp.Topic,
			Partition: ftp.Partition,
			Records:   make([]ConsumedRecord, len(ftp.Records)),
		}

		for i, r := range ftp.Records {
			batch.Records[i] = kgoToConsumedRecord(r)
		}

		result.Batches = append(result.Batches, batch)
	})

	return result
}
