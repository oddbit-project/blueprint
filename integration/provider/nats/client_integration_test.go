//go:build integration
// +build integration

package nats

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Integration test struct for NATS client
type NatsIntegrationTestSuite struct {
	suite.Suite
	ctx       context.Context
	cancel    context.CancelFunc
	producer  *nats.Producer
	consumer  *nats.Consumer
	logger    *log.Logger
	testSubj  string
	queueName string
}

// TestMessage is a simple struct for testing JSON messages
type TestMessage struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	IsActive  bool      `json:"is_active"`
}

// SetupSuite prepares the test environment
func (s *NatsIntegrationTestSuite) SetupSuite() {
	// Create context with cancellation
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Create logger
	s.logger = log.New("nats-integration-test")

	// Set test subject and queue
	s.testSubj = "test.integration"
	s.queueName = "test-queue"

	// Create producer config
	producerConfig := &nats.ProducerConfig{
		URL:      "nats://testuser:testpassword@nats:4222",
		Subject:  s.testSubj,
		AuthType: nats.AuthTypeNone, // Credentials are in URL
	}

	// Create consumer config
	consumerConfig := &nats.ConsumerConfig{
		URL:      "nats://testuser:testpassword@nats:4222",
		Subject:  s.testSubj,
		AuthType: nats.AuthTypeNone, // Credentials are in URL
		ConsumerOptions: nats.ConsumerOptions{
			QueueGroup: s.queueName,
		},
	}

	// Create producer
	var err error
	s.producer, err = nats.NewProducer(producerConfig, s.logger)
	if err != nil {
		s.T().Fatalf("Failed to create NATS producer: %v", err)
	}

	// Create consumer
	s.consumer, err = nats.NewConsumer(consumerConfig, s.logger)
	if err != nil {
		s.T().Fatalf("Failed to create NATS consumer: %v", err)
	}
}

// TearDownSuite cleans up after all tests
func (s *NatsIntegrationTestSuite) TearDownSuite() {
	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Close producer
	if s.producer != nil {
		s.producer.Disconnect()
	}

	// Close consumer
	if s.consumer != nil {
		s.consumer.Disconnect()
	}
}

// TestConnection tests basic connectivity
func (s *NatsIntegrationTestSuite) TestConnection() {
	// Verify producer connection
	assert.True(s.T(), s.producer.IsConnected(), "Producer should be connected")

	// Verify consumer connection
	assert.True(s.T(), s.consumer.IsConnected(), "Consumer should be connected")
}

// TestPublishSubscribe tests basic publish/subscribe functionality
func (s *NatsIntegrationTestSuite) TestPublishSubscribe() {
	// Create a wait group for synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Message to send
	testMessage := "Hello NATS Integration Test!"
	receivedMsg := ""

	// Subscribe to the test subject
	handler := func(ctx context.Context, msg nats.Message) error {
		receivedMsg = string(msg.Data)
		wg.Done()
		return nil
	}

	// Subscribe to test subject
	err := s.consumer.Subscribe(s.ctx, handler)
	assert.NoError(s.T(), err, "Subscribe should succeed")

	// Publish a message
	err = s.producer.Publish([]byte(testMessage))
	assert.NoError(s.T(), err, "Publish should succeed")

	// Wait for the message to be received (with timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Message was received
		assert.Equal(s.T(), testMessage, receivedMsg, "Received message should match sent message")
	case <-time.After(5 * time.Second):
		s.T().Fatal("Timeout waiting for message")
	}
}

// TestJSONMessages tests JSON message serialization
func (s *NatsIntegrationTestSuite) TestJSONMessages() {
	// Create a wait group for synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Create test message
	now := time.Now()
	sentMessage := TestMessage{
		ID:        "test-123",
		Content:   "Test JSON Content",
		Value:     123.45,
		Timestamp: now,
		IsActive:  true,
	}

	var receivedMessage TestMessage

	// Subscribe to the test subject
	handler := func(ctx context.Context, msg nats.Message) error {
		err := json.Unmarshal(msg.Data, &receivedMessage)
		assert.NoError(s.T(), err, "JSON unmarshaling should succeed")
		wg.Done()
		return nil
	}

	// Subscribe to test subject with a unique subject for this test
	jsonSubject := s.testSubj + ".json"
	consumer, err := nats.NewConsumer(&nats.ConsumerConfig{
		URL:      "nats://testuser:testpassword@nats:4222",
		Subject:  jsonSubject,
		AuthType: nats.AuthTypeNone, // Credentials are in URL
	}, s.logger)
	assert.NoError(s.T(), err, "Creating consumer should succeed")
	defer consumer.Disconnect()

	err = consumer.Subscribe(s.ctx, handler)
	assert.NoError(s.T(), err, "Subscribe should succeed")

	// Publish JSON message
	err = s.producer.PublishJSONMsg(jsonSubject, sentMessage)
	assert.NoError(s.T(), err, "PublishJSON should succeed")

	// Wait for the message to be received (with timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Message was received, verify contents
		assert.Equal(s.T(), sentMessage.ID, receivedMessage.ID)
		assert.Equal(s.T(), sentMessage.Content, receivedMessage.Content)
		assert.Equal(s.T(), sentMessage.Value, receivedMessage.Value)
		assert.Equal(s.T(), sentMessage.IsActive, receivedMessage.IsActive)
	case <-time.After(5 * time.Second):
		s.T().Fatal("Timeout waiting for JSON message")
	}
}

// TestRequestReply tests request-reply pattern
func (s *NatsIntegrationTestSuite) TestRequestReply() {
	// Create a unique subject for this test
	requestSubject := s.testSubj + ".request"

	// Start a responder
	var wg sync.WaitGroup
	wg.Add(1)

	// Subscribe to handle requests
	go func() {
		// Create consumer for request handling
		responder, err := nats.NewConsumer(&nats.ConsumerConfig{
			URL:      "nats://testuser:testpassword@nats:4222",
			Subject:  requestSubject,
			AuthType: nats.AuthTypeNone, // Credentials are in URL
		}, s.logger)
		assert.NoError(s.T(), err, "Creating responder should succeed")
		defer responder.Disconnect()

		// Handler for requests
		handler := func(ctx context.Context, msg nats.Message) error {
			// For request-reply, the msg.Reply is the subject to respond to
			if msg.Reply != "" {
				// Send a reply
				err := responder.Conn.Publish(msg.Reply, []byte("Response: "+string(msg.Data)))
				assert.NoError(s.T(), err, "Reply should succeed")
			}
			return nil
		}

		// Subscribe to request subject
		err = responder.Subscribe(s.ctx, handler)
		assert.NoError(s.T(), err, "Subscribe should succeed")

		// Signal that responder is ready
		wg.Done()

		// Keep responder running until context is cancelled
		<-s.ctx.Done()
	}()

	// Wait for responder to be ready
	wg.Wait()

	// Send a request and wait for response
	response, err := s.producer.Request(requestSubject, []byte("Test Request"), 5*time.Second)
	assert.NoError(s.T(), err, "Request should succeed")
	assert.Equal(s.T(), "Response: Test Request", string(response.Data))
}

// TestQueueGroups tests queue group functionality
func (s *NatsIntegrationTestSuite) TestQueueGroups() {
	// Create a unique subject for this test
	queueSubject := s.testSubj + ".queue"
	queueGroup := "test-queue-group"

	var mu sync.Mutex
	receivedCount1 := 0
	receivedCount2 := 0

	// Create two consumers in the same queue group
	consumer1, err := nats.NewConsumer(&nats.ConsumerConfig{
		URL:      "nats://testuser:testpassword@nats:4222",
		Subject:  queueSubject,
		AuthType: nats.AuthTypeNone, // Credentials are in URL
		ConsumerOptions: nats.ConsumerOptions{
			QueueGroup: queueGroup,
		},
	}, s.logger)
	assert.NoError(s.T(), err, "Creating consumer1 should succeed")
	defer consumer1.Disconnect()

	consumer2, err := nats.NewConsumer(&nats.ConsumerConfig{
		URL:      "nats://testuser:testpassword@nats:4222",
		Subject:  queueSubject,
		AuthType: nats.AuthTypeNone, // Credentials are in URL
		ConsumerOptions: nats.ConsumerOptions{
			QueueGroup: queueGroup,
		},
	}, s.logger)
	assert.NoError(s.T(), err, "Creating consumer2 should succeed")
	defer consumer2.Disconnect()

	// Setup handlers for both consumers
	handler1 := func(ctx context.Context, msg nats.Message) error {
		mu.Lock()
		receivedCount1++
		mu.Unlock()
		return nil
	}

	handler2 := func(ctx context.Context, msg nats.Message) error {
		mu.Lock()
		receivedCount2++
		mu.Unlock()
		return nil
	}

	// Subscribe both consumers
	err = consumer1.Subscribe(s.ctx, handler1)
	assert.NoError(s.T(), err, "Subscribe consumer1 should succeed")

	err = consumer2.Subscribe(s.ctx, handler2)
	assert.NoError(s.T(), err, "Subscribe consumer2 should succeed")

	// Send multiple messages
	numMessages := 20
	for i := 0; i < numMessages; i++ {
		err = s.producer.PublishMsg(queueSubject, []byte("Queue Test Message"))
		assert.NoError(s.T(), err, "Publish should succeed")
	}

	// Wait for messages to be processed
	time.Sleep(2 * time.Second)

	// Check that messages were distributed between consumers
	mu.Lock()
	total := receivedCount1 + receivedCount2
	mu.Unlock()

	// We expect all messages to be received and distributed across consumers
	assert.Equal(s.T(), numMessages, total, "All messages should be received")
	
	// Both consumers should have received some messages
	// Note: Distribution isn't guaranteed to be exactly equal
	s.T().Logf("Consumer 1 received: %d, Consumer 2 received: %d", receivedCount1, receivedCount2)
	assert.True(s.T(), receivedCount1 > 0, "Consumer 1 should receive some messages")
	assert.True(s.T(), receivedCount2 > 0, "Consumer 2 should receive some messages")
}

// Run the test suite
func TestNatsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(NatsIntegrationTestSuite))
}