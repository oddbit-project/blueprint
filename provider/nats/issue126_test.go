package nats

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// call126 runs fn and fails the test, instead of the test binary, if it panics
func call126(t *testing.T, fn func() error) error {
	t.Helper()
	var err error
	require.NotPanics(t, func() { err = fn() })
	return err
}

func TestIssue126_NilProducerRequest(t *testing.T) {
	var p *Producer
	err := call126(t, func() error {
		_, err := p.Request("subj", []byte("x"), time.Millisecond)
		return err
	})
	assert.EqualError(t, err, "publisher is nil")

	err = call126(t, func() error {
		_, err := p.RequestJSON("subj", map[string]string{"a": "b"}, time.Millisecond)
		return err
	})
	assert.EqualError(t, err, "publisher is nil")
}

func TestIssue126_NilProducerJSONMarshalError(t *testing.T) {
	var p *Producer
	bad := make(chan int)
	var ute *json.UnsupportedTypeError

	err := call126(t, func() error { return p.PublishJSON(bad) })
	assert.ErrorAs(t, err, &ute)
	err = call126(t, func() error { return p.PublishJSONMsg("subj", bad) })
	assert.ErrorAs(t, err, &ute)
	err = call126(t, func() error {
		_, err := p.RequestJSON("subj", bad, time.Millisecond)
		return err
	})
	assert.ErrorAs(t, err, &ute)
}

func TestIssue126_NilLoggerProducerJSONMarshalError(t *testing.T) {
	p := &Producer{}
	bad := make(chan int)
	var ute *json.UnsupportedTypeError

	err := call126(t, func() error { return p.PublishJSON(bad) })
	assert.ErrorAs(t, err, &ute)
	err = call126(t, func() error { return p.PublishJSONMsg("subj", bad) })
	assert.ErrorAs(t, err, &ute)
	err = call126(t, func() error {
		_, err := p.RequestJSON("subj", bad, time.Millisecond)
		return err
	})
	assert.ErrorAs(t, err, &ute)
}

func TestIssue126_NilJSProducerPublish(t *testing.T) {
	var p *JSProducer
	ctx := context.Background()

	err := call126(t, func() error {
		_, err := p.Publish(ctx, []byte("x"))
		return err
	})
	assert.EqualError(t, err, "publisher is nil")
	err = call126(t, func() error {
		_, err := p.PublishJSON(ctx, map[string]string{"a": "b"})
		return err
	})
	assert.EqualError(t, err, "publisher is nil")
	err = call126(t, func() error {
		_, err := p.PublishJSON(ctx, make(chan int))
		return err
	})
	var ute *json.UnsupportedTypeError
	assert.ErrorAs(t, err, &ute)
}

func TestIssue126_NilConsumerSubscribe(t *testing.T) {
	var c *Consumer
	err := call126(t, func() error {
		return c.Subscribe(context.Background(), func(context.Context, Message) error { return nil })
	})
	assert.ErrorIs(t, err, ErrConsumerClosed)
	err = call126(t, func() error {
		_, err := c.SubscribeSync()
		return err
	})
	assert.ErrorIs(t, err, ErrConsumerClosed)
}

// Guards: non-nil receivers keep their existing errors

func TestIssue126_GuardNonNilReceivers(t *testing.T) {
	ctx := context.Background()
	handler := func(context.Context, Message) error { return nil }

	err := call126(t, func() error { _, err := (&Producer{}).Request("subj", []byte("x"), time.Millisecond); return err })
	assert.ErrorIs(t, err, ErrProducerClosed)
	err = call126(t, func() error {
		_, err := (&Producer{}).RequestJSON("subj", map[string]string{"a": "b"}, time.Millisecond)
		return err
	})
	assert.ErrorIs(t, err, ErrProducerClosed)
	err = call126(t, func() error { return (&Producer{}).PublishJSON(map[string]string{"a": "b"}) })
	assert.ErrorIs(t, err, ErrProducerClosed)

	err = call126(t, func() error { _, err := (&JSProducer{}).Publish(ctx, []byte("x")); return err })
	assert.ErrorIs(t, err, ErrProducerClosed)
	err = call126(t, func() error { _, err := (&JSProducer{}).PublishJSON(ctx, map[string]string{"a": "b"}); return err })
	assert.ErrorIs(t, err, ErrProducerClosed)

	err = call126(t, func() error { return (&Consumer{}).Subscribe(ctx, handler) })
	assert.ErrorIs(t, err, ErrConsumerClosed)
	err = call126(t, func() error { _, err := (&Consumer{}).SubscribeSync(); return err })
	assert.ErrorIs(t, err, ErrConsumerClosed)
}

func TestIssue126_GuardLoggerMarshalError(t *testing.T) {
	bad := make(chan int)
	var ute *json.UnsupportedTypeError
	logged := func(t *testing.T, buf *bytes.Buffer, msg string) {
		t.Helper()
		assert.Contains(t, buf.String(), msg)
		buf.Reset()
	}

	logger, buf := setupTestLogger(t)
	p := &Producer{Logger: logger}
	assert.ErrorAs(t, p.PublishJSON(bad), &ute)
	logged(t, buf, "Failed to marshal JSON for NATS publication")
	assert.ErrorAs(t, p.PublishJSONMsg("subj", bad), &ute)
	logged(t, buf, "Failed to marshal JSON for NATS publication")
	_, err := p.RequestJSON("subj", bad, time.Millisecond)
	assert.ErrorAs(t, err, &ute)
	logged(t, buf, "Failed to marshal JSON for NATS request")

	jp := &JSProducer{Logger: logger}
	_, err = jp.PublishJSON(context.Background(), bad)
	assert.ErrorAs(t, err, &ute)
	logged(t, buf, "Failed to marshal JSON for JetStream publish")
}
