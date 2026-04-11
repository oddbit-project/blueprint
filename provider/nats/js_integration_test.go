package nats

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestJSProducer builds a JetStream producer bound to the suite container.
// It auto-creates the stream so each test owns its own stream/subject pair.
func (s *NATSIntegrationTestSuite) getTestJSProducer(streamName, subject string) *JSProducer {
	cfg := &JSProducerConfig{
		JSConnectionConfig: JSConnectionConfig{
			URL:      s.natsURL,
			AuthType: AuthTypeNone, // credentials are in URL
		},
		Subject: subject,
		Stream: StreamConfig{
			Name:     streamName,
			Subjects: []string{subject + ".>"},
			Storage:  "memory",
			Replicas: 1,
		},
		AutoCreateStream: true,
	}
	p, err := NewJSProducer(cfg, s.logger)
	require.NoError(s.T(), err, "Failed to create JS producer")
	return p
}

// getTestJSConsumer builds a durable JetStream pull consumer for the given
// stream. FilterSubject narrows delivery to a subject under the stream.
func (s *NATSIntegrationTestSuite) getTestJSConsumer(streamName, durable, filterSubject string) *JSConsumer {
	cfg := &JSConsumerConfig{
		JSConnectionConfig: JSConnectionConfig{
			URL:      s.natsURL,
			AuthType: AuthTypeNone,
		},
		StreamName:    streamName,
		Durable:       durable,
		ConsumerName:  durable,
		FilterSubject: filterSubject,
		AckPolicy:     "explicit",
		AckWait:       2 * time.Second,
		MaxDeliver:    5,
		MaxAckPending: 100,
		DeliverPolicy: "all",
	}
	c, err := NewJSConsumer(cfg, s.logger)
	require.NoError(s.T(), err, "Failed to create JS consumer")
	return c
}

// TestJetStreamPublishConsume exercises the happy-path: auto-create stream,
// publish, Consume via callback, and verify Ack'd delivery.
func (s *NATSIntegrationTestSuite) TestJetStreamPublishConsume() {
	streamName := "TEST_JS_PUBCONSUME"
	baseSubject := "js.pubconsume"
	subject := baseSubject + ".msg"

	producer := s.getTestJSProducer(streamName, baseSubject)
	defer producer.Disconnect()

	consumer := s.getTestJSConsumer(streamName, "dur_pubconsume", subject)
	defer consumer.Disconnect()

	const total = 5
	var received int32
	done := make(chan struct{})

	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	err := consumer.Consume(ctx, func(_ context.Context, msg JSMessage) error {
		if string(msg.Data()) == "" {
			return fmt.Errorf("empty payload")
		}
		if atomic.AddInt32(&received, 1) == total {
			close(done)
		}
		return nil
	})
	require.NoError(s.T(), err, "Consume should start")

	for i := 0; i < total; i++ {
		pubCtx, pubCancel := context.WithTimeout(s.ctx, 2*time.Second)
		ack, err := producer.PublishMsg(pubCtx, subject, []byte(fmt.Sprintf("msg-%d", i)))
		pubCancel()
		require.NoError(s.T(), err, "Publish should succeed")
		require.NotNil(s.T(), ack)
		assert.Equal(s.T(), streamName, ack.Stream)
	}

	select {
	case <-done:
		assert.Equal(s.T(), int32(total), atomic.LoadInt32(&received))
	case <-time.After(5 * time.Second):
		s.T().Fatalf("timeout waiting for %d messages, got %d", total, atomic.LoadInt32(&received))
	}
}

// TestJetStreamFetch exercises the one-off pull path.
func (s *NATSIntegrationTestSuite) TestJetStreamFetch() {
	streamName := "TEST_JS_FETCH"
	baseSubject := "js.fetch"
	subject := baseSubject + ".msg"

	producer := s.getTestJSProducer(streamName, baseSubject)
	defer producer.Disconnect()

	consumer := s.getTestJSConsumer(streamName, "dur_fetch", subject)
	defer consumer.Disconnect()

	// Publish before fetching so the messages are already available.
	const total = 3
	for i := 0; i < total; i++ {
		pubCtx, pubCancel := context.WithTimeout(s.ctx, 2*time.Second)
		_, err := producer.PublishMsg(pubCtx, subject, []byte(fmt.Sprintf("fetch-%d", i)))
		pubCancel()
		require.NoError(s.T(), err)
	}

	msgs, err := consumer.Fetch(total, 3*time.Second)
	require.NoError(s.T(), err)
	require.Len(s.T(), msgs, total)

	for _, m := range msgs {
		assert.NotEmpty(s.T(), m.Data())
		require.NoError(s.T(), m.Ack())
		meta, err := m.Metadata()
		require.NoError(s.T(), err)
		assert.Equal(s.T(), streamName, meta.Stream)
	}
}

// TestJetStreamRedelivery verifies that a handler returning an error causes
// the message to be Nak'd and then redelivered up to MaxDeliver.
func (s *NATSIntegrationTestSuite) TestJetStreamRedelivery() {
	streamName := "TEST_JS_REDELIVER"
	baseSubject := "js.redeliver"
	subject := baseSubject + ".msg"

	producer := s.getTestJSProducer(streamName, baseSubject)
	defer producer.Disconnect()

	consumer := s.getTestJSConsumer(streamName, "dur_redeliver", subject)
	defer consumer.Disconnect()

	var attempts int32
	delivered := make(chan struct{}, 1)

	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()

	err := consumer.Consume(ctx, func(_ context.Context, msg JSMessage) error {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			// first attempt: force redelivery
			return fmt.Errorf("simulated failure attempt %d", n)
		}
		select {
		case delivered <- struct{}{}:
		default:
		}
		return nil
	})
	require.NoError(s.T(), err)

	pubCtx, pubCancel := context.WithTimeout(s.ctx, 2*time.Second)
	_, err = producer.PublishMsg(pubCtx, subject, []byte("retry-me"))
	pubCancel()
	require.NoError(s.T(), err)

	select {
	case <-delivered:
		assert.GreaterOrEqual(s.T(), atomic.LoadInt32(&attempts), int32(2))
	case <-time.After(10 * time.Second):
		s.T().Fatalf("message not redelivered, attempts=%d", atomic.LoadInt32(&attempts))
	}
}

// TestJetStreamEnsureStreamExplicit verifies the EnsureStream helper path when
// AutoCreateStream is left false on the producer.
func (s *NATSIntegrationTestSuite) TestJetStreamEnsureStreamExplicit() {
	streamName := "TEST_JS_EXPLICIT"
	baseSubject := "js.explicit"
	subject := baseSubject + ".msg"

	// First, create the stream via a producer with AutoCreateStream so that a
	// second, non-creating producer can find it.
	bootstrap := s.getTestJSProducer(streamName, baseSubject)
	bootstrap.Disconnect()

	cfg := &JSProducerConfig{
		JSConnectionConfig: JSConnectionConfig{
			URL:      s.natsURL,
			AuthType: AuthTypeNone,
		},
		Subject: subject,
		Stream: StreamConfig{
			Name: streamName,
		},
		AutoCreateStream: false,
	}
	p, err := NewJSProducer(cfg, s.logger)
	require.NoError(s.T(), err)
	defer p.Disconnect()

	require.NotNil(s.T(), p.Stream, "Stream should have been looked up")

	pubCtx, pubCancel := context.WithTimeout(s.ctx, 2*time.Second)
	ack, err := p.PublishMsg(pubCtx, subject, []byte("hello"))
	pubCancel()
	require.NoError(s.T(), err)
	assert.Equal(s.T(), streamName, ack.Stream)
}

// TestJetStreamMissingStreamName guards the validation path.
func TestJetStreamMissingStreamName(t *testing.T) {
	_, err := NewJSConsumer(&JSConsumerConfig{
		JSConnectionConfig: JSConnectionConfig{
			URL:      "nats://localhost:4222",
			AuthType: AuthTypeNone,
		},
	}, nil)
	assert.ErrorIs(t, err, ErrMissingStreamName)
}

// TestJSConsumerConfigValidate covers the fast-fail validation added to avoid
// opening a connection and performing stream lookups on invalid configs.
func TestJSConsumerConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     JSConsumerConfig
		wantErr error
	}{
		{
			name:    "missing URL",
			cfg:     JSConsumerConfig{},
			wantErr: ErrMissingJSURL,
		},
		{
			name: "invalid auth type",
			cfg: JSConsumerConfig{
				JSConnectionConfig: JSConnectionConfig{URL: "nats://x", AuthType: "bogus"},
			},
			wantErr: ErrInvalidAuthType,
		},
		{
			name: "missing stream",
			cfg: JSConsumerConfig{
				JSConnectionConfig: JSConnectionConfig{URL: "nats://x", AuthType: AuthTypeNone},
			},
			wantErr: ErrMissingStreamName,
		},
		{
			name: "invalid ack policy",
			cfg: JSConsumerConfig{
				JSConnectionConfig: JSConnectionConfig{URL: "nats://x", AuthType: AuthTypeNone},
				StreamName:         "S",
				AckPolicy:          "whatever",
			},
			wantErr: ErrInvalidAckPolicy,
		},
		{
			name: "invalid deliver policy",
			cfg: JSConsumerConfig{
				JSConnectionConfig: JSConnectionConfig{URL: "nats://x", AuthType: AuthTypeNone},
				StreamName:         "S",
				DeliverPolicy:      "sometime",
			},
			wantErr: ErrInvalidDeliverPolicy,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// TestJSProducerConfigValidate covers producer-side fast-fail validation.
func TestJSProducerConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     JSProducerConfig
		wantErr error
	}{
		{
			name:    "missing URL",
			cfg:     JSProducerConfig{},
			wantErr: ErrMissingJSURL,
		},
		{
			name: "missing subject",
			cfg: JSProducerConfig{
				JSConnectionConfig: JSConnectionConfig{URL: "nats://x", AuthType: AuthTypeNone},
			},
			wantErr: ErrMissingProducerTopic,
		},
		{
			name: "auto create with bad retention",
			cfg: JSProducerConfig{
				JSConnectionConfig: JSConnectionConfig{URL: "nats://x", AuthType: AuthTypeNone},
				Subject:            "s",
				Stream:             StreamConfig{Name: "S", Retention: "bogus"},
				AutoCreateStream:   true,
			},
			wantErr: ErrInvalidRetention,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// TestJetStreamConsumeTwiceRejected verifies that a second Consume() call on
// the same JSConsumer is rejected with ErrAlreadyConsuming rather than
// silently overwriting the first consume context.
func (s *NATSIntegrationTestSuite) TestJetStreamConsumeTwiceRejected() {
	streamName := "TEST_JS_CONSUMETWICE"
	baseSubject := "js.twice"
	subject := baseSubject + ".msg"

	producer := s.getTestJSProducer(streamName, baseSubject)
	defer producer.Disconnect()

	consumer := s.getTestJSConsumer(streamName, "dur_twice", subject)
	defer consumer.Disconnect()

	ctx1, cancel1 := context.WithCancel(s.ctx)
	defer cancel1()

	err := consumer.Consume(ctx1, func(_ context.Context, _ JSMessage) error { return nil })
	require.NoError(s.T(), err)

	err = consumer.Consume(s.ctx, func(_ context.Context, _ JSMessage) error { return nil })
	assert.ErrorIs(s.T(), err, ErrAlreadyConsuming)

	// After cancelling the first context the watcher goroutine should release
	// consCt, allowing a subsequent Consume() to succeed. Poll briefly for
	// the teardown since it runs asynchronously.
	cancel1()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err = consumer.Consume(s.ctx, func(_ context.Context, _ JSMessage) error { return nil })
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NoError(s.T(), err, "second Consume after first ctx cancel should succeed")
}

// TestJetStreamConsumeThenDisconnect verifies that Disconnect() cleanly stops
// an active Consume session even when the caller's context is a non-cancellable
// background context (H2 regression guard — without the stop channel, the
// watcher goroutine would leak).
func (s *NATSIntegrationTestSuite) TestJetStreamConsumeThenDisconnect() {
	streamName := "TEST_JS_DISCONNECT"
	baseSubject := "js.disconnect"
	subject := baseSubject + ".msg"

	producer := s.getTestJSProducer(streamName, baseSubject)
	defer producer.Disconnect()

	consumer := s.getTestJSConsumer(streamName, "dur_disconnect", subject)

	err := consumer.Consume(context.Background(), func(_ context.Context, _ JSMessage) error {
		return nil
	})
	require.NoError(s.T(), err)

	// Disconnect immediately — the watcher goroutine should be released via
	// the internal stop channel even though the background context never
	// cancels.
	consumer.Disconnect()
	assert.False(s.T(), consumer.IsConnected())

	// Second Disconnect should be a no-op.
	consumer.Disconnect()
}
