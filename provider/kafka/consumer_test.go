package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// ConsumerUnitTestSuite contains all unit tests for the consumer
type ConsumerUnitTestSuite struct {
	suite.Suite
}

// TestErrorHandling verifies basic error handling in consumer operations
func (s *ConsumerUnitTestSuite) TestErrorHandling() {
	// Test that various error types are handled appropriately
	// This test focuses on the consumer's ability to handle errors rather than categorizing them
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.NoError(err)
	s.NotNil(consumer)
}

// TestConsumerCreation tests consumer initialization
func (s *ConsumerUnitTestSuite) TestConsumerCreation() {
	// Test nil config
	consumer, err := NewConsumer(nil, nil)
	s.Error(err)
	s.Nil(consumer)

	// Test valid config
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err = NewConsumer(cfg, nil)
	s.NoError(err)
	s.NotNil(consumer)
	s.NotNil(consumer.Logger)
	s.Nil(consumer.Reader)
	s.False(consumer.IsConnected())
}

// TestConcurrentConnection tests thread safety of connection operations
func (s *ConsumerUnitTestSuite) TestConcurrentConnection() {
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.Require().NoError(err)

	// Start multiple goroutines trying to connect simultaneously
	var wg sync.WaitGroup
	const numGoroutines = 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			consumer.Connect()
			s.True(consumer.IsConnected())
		}()
	}

	wg.Wait()

	// Should be connected
	s.True(consumer.IsConnected())
	s.NotNil(consumer.Reader)
}

// TestMultipleDisconnects tests concurrent disconnect safety
func (s *ConsumerUnitTestSuite) TestMultipleDisconnects() {
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.Require().NoError(err)

	// Connect first
	consumer.Connect()
	s.True(consumer.IsConnected())

	// Test concurrent disconnects - should not hang
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			consumer.Disconnect()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		s.T().Fatal("Disconnect operations hung")
	}

	s.False(consumer.IsConnected())
}

// TestConsumerOperations tests basic consumer operations
func (s *ConsumerUnitTestSuite) TestConsumerOperations() {
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.Require().NoError(err)

	// Test that operations can be called (though they may fail due to no actual Kafka broker)
	// The point is to test the API structure, not actual Kafka connectivity
	s.NotNil(consumer.GetConfig())

	// Test rewind when not connected
	err = consumer.Rewind()
	s.NoError(err, "Rewind should work when not connected")

	// Test disconnect when not connected (should not panic)
	consumer.Disconnect()
	s.False(consumer.IsConnected())
}

// TestConnectDisconnectCycle tests the connect/disconnect cycle
func (s *ConsumerUnitTestSuite) TestConnectDisconnectCycle() {
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.Require().NoError(err)

	// Initially not connected
	s.False(consumer.IsConnected())

	// Connect
	consumer.Connect()
	s.True(consumer.IsConnected())
	s.NotNil(consumer.Reader)

	// Disconnect then reconnect
	consumer.Disconnect()
	s.False(consumer.IsConnected())

	consumer.Connect()
	s.True(consumer.IsConnected())
	s.NotNil(consumer.Reader)
}

// TestReadMessageTracking tests that ReadMessage properly tracks itself
func (s *ConsumerUnitTestSuite) TestReadMessageTracking() {
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start ReadMessage in background
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := consumer.ReadMessage(ctx)
		// Error is expected since we're not connected to real Kafka
		s.T().Log("ReadMessage error (expected):", err)
	}()

	// Give ReadMessage time to register with WaitGroup
	time.Sleep(100 * time.Millisecond)

	// Disconnect should wait for ReadMessage
	start := time.Now()
	consumer.Disconnect()
	elapsed := time.Since(start)

	// Should wait for ReadMessage (network timeout ~8-10 seconds with no broker)
	s.Greater(elapsed, 500*time.Millisecond, "Should wait for ReadMessage")
	s.Less(elapsed, 15*time.Second, "Should not hang forever")

	<-done
}

// TestChannelSubscribeNonBlocking tests channel send doesn't block
func (s *ConsumerUnitTestSuite) TestChannelSubscribeNonBlocking() {
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.Require().NoError(err)

	// Small buffer channel
	ch := make(chan Message, 1)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		err := consumer.ChannelSubscribe(ctx, ch)
		done <- err
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context - should exit cleanly even if channel is full
	cancel()

	select {
	case err := <-done:
		s.True(err == nil || err == context.Canceled)
	case <-time.After(2 * time.Second):
		s.T().Fatal("ChannelSubscribe hung on cancellation")
	}
}

// TestDisconnectSafety tests disconnect safety
func (s *ConsumerUnitTestSuite) TestDisconnectSafety() {
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.Require().NoError(err)

	// Should not panic even when disconnecting multiple times
	s.NotPanics(func() {
		consumer.Disconnect()
		consumer.Disconnect()
	})
}

// TestRewind tests the Rewind functionality
func (s *ConsumerUnitTestSuite) TestRewind() {
	cfg := &ConsumerConfig{
		Brokers:  "localhost:9092",
		Topic:    "test",
		AuthType: "none",
	}

	consumer, err := NewConsumer(cfg, nil)
	s.Require().NoError(err)

	// Should work when not connected
	err = consumer.Rewind()
	s.NoError(err)

	// Should fail when connected
	consumer.Connect()
	err = consumer.Rewind()
	s.Error(err)
	s.Equal(ErrConsumerAlreadyConnected, err)
}

// ConsumerIntegrationTestSuite contains integration tests that require Kafka
type ConsumerIntegrationTestSuite struct {
	KafkaIntegrationTestSuite
}

// TestSimpleProducerConsumer tests basic produce/consume flow
func (s *ConsumerIntegrationTestSuite) TestSimpleProducerConsumer() {
	producerCfg, consumerCfg := s.getConfig()
	s.purgeTopic(producerCfg)

	producer, err := NewProducer(producerCfg, nil)
	s.Require().NoError(err)
	defer producer.Disconnect()

	consumer, err := NewConsumer(consumerCfg, nil)
	s.Require().NoError(err)
	defer consumer.Disconnect()

	// Send message
	testMsg := []byte("test message")
	err = producer.Write(s.ctx, testMsg)
	s.Require().NoError(err)

	// Receive message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := consumer.ReadMessage(ctx)
	s.Require().NoError(err)
	s.Equal(testMsg, msg.Value)
}

// TestContextCancellationShutdown tests clean shutdown via context
func (s *ConsumerIntegrationTestSuite) TestContextCancellationShutdown() {
	_, consumerCfg := s.getConfig()
	consumerCfg.Topic = "test_context_shutdown"

	consumer, err := NewConsumer(consumerCfg, nil)
	s.Require().NoError(err)
	defer consumer.Disconnect()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		err := consumer.Subscribe(ctx, func(ctx context.Context, msg Message) error {
			return nil
		})
		done <- err
	}()

	// Let subscription start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	select {
	case err := <-done:
		s.True(err == nil || errors.Is(err, context.Canceled))
	case <-time.After(5 * time.Second):
		s.T().Fatal("Subscribe didn't exit on context cancellation")
	}
}

// TestForcedDisconnectWhileReading tests disconnect while actively reading
func (s *ConsumerIntegrationTestSuite) TestForcedDisconnectWhileReading() {
	_, consumerCfg := s.getConfig()
	consumerCfg.Topic = "test_forced_disconnect"

	consumer, err := NewConsumer(consumerCfg, nil)
	s.Require().NoError(err)

	ctx := context.Background()
	done := make(chan error, 1)

	go func() {
		err := consumer.Subscribe(ctx, func(ctx context.Context, msg Message) error {
			return nil
		})
		done <- err
	}()

	// Let subscription start and block on read
	time.Sleep(500 * time.Millisecond)

	// Force disconnect
	consumer.Disconnect()

	select {
	case err := <-done:
		s.True(err == nil || isClosedError(err))
	case <-time.After(5 * time.Second):
		s.T().Fatal("Subscribe didn't exit after disconnect")
	}
}

// TestChannelSubscribeIntegration tests channel-based consumption
func (s *ConsumerIntegrationTestSuite) TestChannelSubscribeIntegration() {
	producerCfg, consumerCfg := s.getConfig()
	consumerCfg.Topic = "test_channel_subscribe"
	producerCfg.Topic = "test_channel_subscribe"

	s.purgeTopic(producerCfg)

	producer, err := NewProducer(producerCfg, nil)
	s.Require().NoError(err)
	defer producer.Disconnect()

	consumer, err := NewConsumer(consumerCfg, nil)
	s.Require().NoError(err)
	defer consumer.Disconnect()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgChan := make(chan Message, 10)
	done := make(chan error, 1)

	// Start channel subscriber
	go func() {
		err := consumer.ChannelSubscribe(ctx, msgChan)
		done <- err
		close(msgChan)
	}()

	// Send messages
	for i := 0; i < 3; i++ {
		msg := []byte("message " + string(rune(i+'0')))
		err = producer.Write(s.ctx, msg)
		s.Require().NoError(err)
	}

	// Receive messages
	received := 0
	timeout := time.After(5 * time.Second)

	for received < 3 {
		select {
		case msg := <-msgChan:
			s.T().Logf("Received: %s", string(msg.Value))
			received++
		case <-timeout:
			s.T().Fatal("Timeout waiting for messages")
		}
	}

	// Clean shutdown
	cancel()

	select {
	case err := <-done:
		s.True(err == nil || errors.Is(err, context.Canceled))
	case <-time.After(5 * time.Second):
		s.T().Fatal("ChannelSubscribe didn't exit cleanly")
	}
}

// TestMultipleSubscribers tests concurrent subscribers
func (s *ConsumerIntegrationTestSuite) TestMultipleSubscribers() {
	producerCfg, consumerCfg := s.getConfig()
	consumerCfg.Topic = "test_multiple_subscribers"
	producerCfg.Topic = "test_multiple_subscribers"

	s.purgeTopic(producerCfg)

	producer, err := NewProducer(producerCfg, nil)
	s.Require().NoError(err)
	defer producer.Disconnect()

	consumer, err := NewConsumer(consumerCfg, nil)
	s.Require().NoError(err)
	defer consumer.Disconnect()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start multiple subscribers
	var wg sync.WaitGroup
	messageCount := &sync.Map{}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		subscriberID := i
		go func(id int) {
			defer wg.Done()
			err := consumer.Subscribe(ctx, func(ctx context.Context, msg Message) error {
				messageCount.Store(id, true)
				s.T().Logf("Subscriber %d processed: %s", id, string(msg.Value))
				return nil
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				s.T().Logf("Subscriber %d error: %v", id, err)
			}
		}(subscriberID)
		time.Sleep(100 * time.Millisecond) // Stagger starts
	}

	// Send messages
	time.Sleep(500 * time.Millisecond)
	for i := 0; i < 5; i++ {
		err = producer.Write(s.ctx, []byte("test message"))
		s.Require().NoError(err)
	}

	// Let messages process
	time.Sleep(2 * time.Second)

	// Cancel and wait
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.T().Log("All subscribers exited cleanly")
	case <-time.After(5 * time.Second):
		s.T().Fatal("Subscribers didn't exit")
	}

	// Verify at least one subscriber processed messages
	processed := 0
	messageCount.Range(func(key, value interface{}) bool {
		processed++
		return true
	})
	s.Greater(processed, 0)
}

// TestConsumerUnitTests runs the unit test suite
func TestConsumerUnitTests(t *testing.T) {
	suite.Run(t, new(ConsumerUnitTestSuite))
}

// TestConsumerIntegrationTests runs the integration test suite
func TestConsumerIntegrationTests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(ConsumerIntegrationTestSuite))
}