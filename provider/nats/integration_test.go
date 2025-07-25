package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestMessage is a simple struct for testing JSON messages
type TestMessage struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	IsActive  bool      `json:"is_active"`
}

// NATSIntegrationTestSuite manages the NATS testcontainer and provides comprehensive testing
type NATSIntegrationTestSuite struct {
	suite.Suite
	ctx       context.Context
	cancel    context.CancelFunc
	container testcontainers.Container
	natsURL   string
	logger    *log.Logger
	testSubj  string
	queueName string
}

// SetupSuite prepares the test environment with testcontainers
func (s *NATSIntegrationTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Create logger
	s.logger = log.New("nats-integration-test")

	// Set test subject and queue
	s.testSubj = "test.integration"
	s.queueName = "test-queue"

	// Create NATS testcontainer
	var err error
	s.container, err = nats.Run(s.ctx,
		"nats:2.10-alpine",
		nats.WithUsername("testuser"),
		nats.WithPassword("testpassword"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("4222/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(s.T(), err, "Failed to start NATS container")

	// Get connection URL
	host, err := s.container.Host(s.ctx)
	require.NoError(s.T(), err, "Failed to get NATS host")

	mappedPort, err := s.container.MappedPort(s.ctx, "4222/tcp")
	require.NoError(s.T(), err, "Failed to get NATS port")

	s.natsURL = fmt.Sprintf("nats://testuser:testpassword@%s:%s", host, mappedPort.Port())
	s.T().Logf("NATS container started with URL: %s", s.natsURL)
}

// TearDownSuite cleans up after all tests
func (s *NATSIntegrationTestSuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}

	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		if err != nil {
			s.T().Logf("Failed to terminate NATS container: %v", err)
		}
	}
}

// getTestProducer creates a producer using the testcontainer URL
func (s *NATSIntegrationTestSuite) getTestProducer(subject string) *Producer {
	cfg := &ProducerConfig{
		URL:      s.natsURL,
		Subject:  subject,
		AuthType: AuthTypeNone, // Credentials are in URL
	}
	producer, err := NewProducer(cfg, s.logger)
	require.NoError(s.T(), err, "Failed to create test producer")
	return producer
}

// getTestConsumer creates a consumer using the testcontainer URL
func (s *NATSIntegrationTestSuite) getTestConsumer(subject string, queueGroup ...string) *Consumer {
	cfg := &ConsumerConfig{
		URL:      s.natsURL,
		Subject:  subject,
		AuthType: AuthTypeNone, // Credentials are in URL
	}
	if len(queueGroup) > 0 {
		cfg.ConsumerOptions = ConsumerOptions{
			QueueGroup: queueGroup[0],
		}
	}
	consumer, err := NewConsumer(cfg, s.logger)
	require.NoError(s.T(), err, "Failed to create test consumer")
	return consumer
}

// TestConnection tests basic connectivity
func (s *NATSIntegrationTestSuite) TestConnection() {
	producer := s.getTestProducer(s.testSubj)
	defer producer.Disconnect()

	consumer := s.getTestConsumer(s.testSubj)
	defer consumer.Disconnect()

	assert.True(s.T(), producer.IsConnected(), "Producer should be connected")
	assert.True(s.T(), consumer.IsConnected(), "Consumer should be connected")
}

// TestPublishSubscribe tests basic publish/subscribe functionality
func (s *NATSIntegrationTestSuite) TestPublishSubscribe() {
	producer := s.getTestProducer(s.testSubj)
	defer producer.Disconnect()

	consumer := s.getTestConsumer(s.testSubj)
	defer consumer.Disconnect()

	// Create a wait group for synchronization
	var wg sync.WaitGroup
	wg.Add(1)

	// Message to send
	testMessage := "Hello NATS Integration Test!"
	receivedMsg := ""

	// Subscribe to the test subject
	handler := func(ctx context.Context, msg Message) error {
		receivedMsg = string(msg.Data)
		wg.Done()
		return nil
	}

	// Subscribe to test subject
	err := consumer.Subscribe(s.ctx, handler)
	require.NoError(s.T(), err, "Subscribe should succeed")

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish a message
	err = producer.Publish([]byte(testMessage))
	require.NoError(s.T(), err, "Publish should succeed")

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
func (s *NATSIntegrationTestSuite) TestJSONMessages() {
	jsonSubject := s.testSubj + ".json"
	producer := s.getTestProducer(jsonSubject)
	defer producer.Disconnect()

	consumer := s.getTestConsumer(jsonSubject)
	defer consumer.Disconnect()

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
	handler := func(ctx context.Context, msg Message) error {
		err := json.Unmarshal(msg.Data, &receivedMessage)
		require.NoError(s.T(), err, "JSON unmarshaling should succeed")
		wg.Done()
		return nil
	}

	err := consumer.Subscribe(s.ctx, handler)
	require.NoError(s.T(), err, "Subscribe should succeed")

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish JSON message
	err = producer.PublishJSONMsg(jsonSubject, sentMessage)
	require.NoError(s.T(), err, "PublishJSON should succeed")

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
func (s *NATSIntegrationTestSuite) TestRequestReply() {
	requestSubject := s.testSubj + ".request"
	producer := s.getTestProducer(requestSubject)
	defer producer.Disconnect()

	responder := s.getTestConsumer(requestSubject)
	defer responder.Disconnect()

	// Create a separate context for this test to avoid cancellation issues
	testCtx, testCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer testCancel()

	// Start a responder
	var wg sync.WaitGroup
	wg.Add(1)

	// Channel to signal when responder is fully ready
	responderReady := make(chan struct{})
	responderDone := make(chan struct{})

	// Subscribe to handle requests
	go func() {
		defer close(responderDone)

		// Handler for requests
		handler := func(ctx context.Context, msg Message) error {
			// For request-reply, the msg.Reply is the subject to respond to
			if msg.Reply != "" {
				// Send a reply
				err := responder.Conn.Publish(msg.Reply, []byte("Response: "+string(msg.Data)))
				require.NoError(s.T(), err, "Reply should succeed")
			}
			return nil
		}

		// Subscribe to request subject
		err := responder.Subscribe(testCtx, handler)
		require.NoError(s.T(), err, "Subscribe should succeed")

		// Signal that responder is ready
		wg.Done()
		close(responderReady)

		// Keep responder running until test context is cancelled
		<-testCtx.Done()
	}()

	// Wait for responder to be ready
	wg.Wait()

	// Wait for responder to be fully subscribed
	select {
	case <-responderReady:
		// Responder is ready
	case <-time.After(5 * time.Second):
		s.T().Fatal("Responder not ready within timeout")
		return
	}

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Send a request and wait for response
	response, err := producer.Request(requestSubject, []byte("Test Request"), 5*time.Second)
	require.NoError(s.T(), err, "Request should succeed")
	require.NotNil(s.T(), response, "Response should not be nil")
	assert.Equal(s.T(), "Response: Test Request", string(response.Data))

	// Wait for responder goroutine to finish cleanly
	testCancel()
	select {
	case <-responderDone:
		// Responder finished cleanly
	case <-time.After(2 * time.Second):
		s.T().Log("Responder did not finish within timeout")
	}
}

// TestQueueGroups tests queue group functionality
func (s *NATSIntegrationTestSuite) TestQueueGroups() {
	queueSubject := s.testSubj + ".queue"
	queueGroup := "test-queue-group"

	producer := s.getTestProducer(queueSubject)
	defer producer.Disconnect()

	var mu sync.Mutex
	receivedCount1 := 0
	receivedCount2 := 0

	// Create two consumers in the same queue group
	consumer1 := s.getTestConsumer(queueSubject, queueGroup)
	defer consumer1.Disconnect()

	consumer2 := s.getTestConsumer(queueSubject, queueGroup)
	defer consumer2.Disconnect()

	// Setup handlers for both consumers
	handler1 := func(ctx context.Context, msg Message) error {
		mu.Lock()
		receivedCount1++
		mu.Unlock()
		return nil
	}

	handler2 := func(ctx context.Context, msg Message) error {
		mu.Lock()
		receivedCount2++
		mu.Unlock()
		return nil
	}

	// Subscribe both consumers
	err := consumer1.Subscribe(s.ctx, handler1)
	require.NoError(s.T(), err, "Subscribe should succeed for consumer1")

	err = consumer2.Subscribe(s.ctx, handler2)
	require.NoError(s.T(), err, "Subscribe should succeed for consumer2")

	// Give subscriptions time to establish
	time.Sleep(100 * time.Millisecond)

	// Send multiple messages
	numMessages := 20
	for i := 0; i < numMessages; i++ {
		err = producer.PublishMsg(queueSubject, []byte("Queue Test Message"))
		require.NoError(s.T(), err, "Publish should succeed")
	}

	// Wait for messages to be processed
	time.Sleep(2 * time.Second)

	// Check that messages were distributed between consumers
	mu.Lock()
	total := receivedCount1 + receivedCount2
	mu.Unlock()

	// Log message distribution
	s.T().Logf("Consumer 1 received: %d, Consumer 2 received: %d (total: %d of %d sent)",
		receivedCount1, receivedCount2, total, numMessages)

	// Verify that messages were received and distributed
	assert.Equal(s.T(), numMessages, total, "All messages should be received")
	// In queue groups, messages should be distributed (not duplicated)
	// Each message should only be received by one consumer
	assert.True(s.T(), receivedCount1 > 0 || receivedCount2 > 0, "At least one consumer should receive messages")
}

// TestBasicAuth tests basic authentication
func (s *NATSIntegrationTestSuite) TestBasicAuth() {
	// Get host and port from container
	host, err := s.container.Host(s.ctx)
	require.NoError(s.T(), err)

	mappedPort, err := s.container.MappedPort(s.ctx, "4222/tcp")
	require.NoError(s.T(), err)

	// Create producer with basic auth
	producerConfig := &ProducerConfig{
		URL:                     fmt.Sprintf("nats://%s:%s", host, mappedPort.Port()),
		Subject:                 "test.auth.basic",
		AuthType:                AuthTypeBasic,
		Username:                "testuser",
		DefaultCredentialConfig: StringPasswordConfig("testpassword"),
	}

	producer, err := NewProducer(producerConfig, s.logger)
	require.NoError(s.T(), err, "Should create producer with basic auth")
	defer producer.Disconnect()

	assert.True(s.T(), producer.IsConnected(), "Producer should be connected with basic auth")

	// Test with invalid credentials - this should fail
	invalidConfig := &ProducerConfig{
		URL:                     fmt.Sprintf("nats://%s:%s", host, mappedPort.Port()),
		Subject:                 "test.auth.basic",
		AuthType:                AuthTypeBasic,
		Username:                "testuser",
		DefaultCredentialConfig: StringPasswordConfig("wrongpassword"),
	}

	invalidProducer, err := NewProducer(invalidConfig, s.logger)
	if err == nil {
		defer invalidProducer.Disconnect()
		assert.False(s.T(), invalidProducer.IsConnected(), "Producer should not connect with invalid basic auth")
	} else {
		// This is the expected outcome - connection should fail with wrong credentials
		s.T().Logf("Correctly failed to connect with invalid credentials: %v", err)
	}
}

// TestDirectURLAuth tests authentication with credentials in URL
func (s *NATSIntegrationTestSuite) TestDirectURLAuth() {
	// Create producer with credentials in URL (this is what we're already doing in other tests)
	producerConfig := &ProducerConfig{
		URL:      s.natsURL,
		Subject:  "test.auth.url",
		AuthType: AuthTypeNone, // Auth is in URL
	}

	producer, err := NewProducer(producerConfig, s.logger)
	require.NoError(s.T(), err, "Should create producer with URL auth")
	defer producer.Disconnect()

	assert.True(s.T(), producer.IsConnected(), "Producer should be connected with URL auth")

	// Test publishing a message
	err = producer.Publish([]byte("URL Auth Test"))
	require.NoError(s.T(), err, "Publishing with URL auth should succeed")
}

// TestConnectionTimeout tests connection timeout handling
func (s *NATSIntegrationTestSuite) TestConnectionTimeout() {
	// Create producer with non-existent server and short timeout
	producerConfig := &ProducerConfig{
		URL:      "nats://nonexistent-host:4222", // Intentionally using a nonexistent host
		Subject:  "test.timeout",
		AuthType: AuthTypeNone,
		ProducerOptions: ProducerOptions{
			Timeout: 500, // 500ms timeout
		},
	}

	startTime := time.Now()
	_, err := NewProducer(producerConfig, s.logger)
	duration := time.Since(startTime)

	// Should fail quickly due to timeout
	assert.Error(s.T(), err, "Connection to nonexistent server should fail")
	assert.Less(s.T(), duration, 2*time.Second, "Connection should time out quickly")
}

// Run the test suite
func TestNatsIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(NATSIntegrationTestSuite))
}
