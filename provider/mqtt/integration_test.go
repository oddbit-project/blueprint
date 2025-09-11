package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
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

// MQTTIntegrationTestSuite manages the MQTT testcontainer and provides comprehensive testing
type MQTTIntegrationTestSuite struct {
	suite.Suite
	ctx       context.Context
	cancel    context.CancelFunc
	container testcontainers.Container
	broker    string
	testTopic string
}

// SetupSuite prepares the test environment with testcontainers
func (s *MQTTIntegrationTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Set test topic
	s.testTopic = "blueprint/test"

	// Create Mosquitto testcontainer
	req := testcontainers.ContainerRequest{
		Image:        "eclipse-mosquitto:2.0.20",
		ExposedPorts: []string{"1883/tcp"},
		Cmd: []string{
			"sh", "-c",
			"echo 'listener 1883\nallow_anonymous true\nlog_dest stdout' > /mosquitto/config/mosquitto.conf && mosquitto -c /mosquitto/config/mosquitto.conf",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("1883/tcp"),
			wait.ForLog("Opening ipv4 listen socket on port 1883").WithStartupTimeout(30*time.Second),
		).WithStartupTimeout(60 * time.Second),
	}

	var err error
	s.container, err = testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(s.T(), err, "Failed to start Mosquitto container")

	// Get connection details
	host, err := s.container.Host(s.ctx)
	require.NoError(s.T(), err, "Failed to get Mosquitto host")

	mappedPort, err := s.container.MappedPort(s.ctx, "1883/tcp")
	require.NoError(s.T(), err, "Failed to get Mosquitto port")

	s.broker = fmt.Sprintf("%s:%s", host, mappedPort.Port())
	s.T().Logf("Mosquitto container started at: %s", s.broker)
}

// TearDownSuite cleans up after all tests
func (s *MQTTIntegrationTestSuite) TearDownSuite() {
	// Terminate container first with its own context before cancelling the main context
	if s.container != nil {
		// Create a separate context for cleanup to avoid cancellation issues
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		
		err := s.container.Terminate(cleanupCtx)
		if err != nil {
			s.T().Logf("Failed to terminate Mosquitto container: %v", err)
		}
	}

	// Cancel the main context after container cleanup
	if s.cancel != nil {
		s.cancel()
	}
}

// getTestConfig creates a test configuration using the testcontainer broker
func (s *MQTTIntegrationTestSuite) getTestConfig() *Config {
	cfg := NewConfig()
	cfg.Brokers = []string{s.broker}
	cfg.Protocol = "tcp"
	cfg.QoS = 0
	cfg.Retain = false
	cfg.PersistentSession = false
	// No username/password for basic test setup
	return cfg
}

// getTestConfigWithAuth creates a test configuration with auth (if supported)
func (s *MQTTIntegrationTestSuite) getTestConfigWithAuth() *Config {
	cfg := s.getTestConfig()
	// Note: Basic Mosquitto container doesn't have auth by default
	// This is for demonstration of the API
	cfg.Username = "testuser"
	cfg.Password = "testpassword"
	return cfg
}

// TestConnection tests basic connectivity
func (s *MQTTIntegrationTestSuite) TestConnection() {
	cfg := s.getTestConfig()
	client, err := NewClient(cfg)
	require.NoError(s.T(), err, "Failed to create MQTT client")
	defer client.Close()

	connected, err := client.Connect()
	require.NoError(s.T(), err, "Failed to connect to MQTT broker")
	assert.True(s.T(), client.Client.IsConnected(), "Client should be connected")
	s.T().Logf("Connection successful, session present: %v", connected)
}

// TestPublishSubscribe tests basic publish/subscribe functionality
func (s *MQTTIntegrationTestSuite) TestPublishSubscribe() {
	cfg := s.getTestConfig()
	client, err := NewClient(cfg)
	require.NoError(s.T(), err, "Failed to create MQTT client")
	defer client.Close()

	_, err = client.Connect()
	require.NoError(s.T(), err, "Failed to connect to MQTT broker")

	// Test message
	testMessage := []byte("Hello MQTT Integration Test!")
	var receivedMessage []byte
	var wg sync.WaitGroup
	wg.Add(1)

	// Subscribe to test topic
	err = client.Subscribe(s.testTopic, 0, func(c paho.Client, msg paho.Message) {
		receivedMessage = msg.Payload()
		wg.Done()
	})
	require.NoError(s.T(), err, "Subscribe should succeed")

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish message
	err = client.Write(s.testTopic, testMessage)
	require.NoError(s.T(), err, "Publish should succeed")

	// Wait for message to be received
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		assert.Equal(s.T(), testMessage, receivedMessage, "Received message should match sent message")
	case <-time.After(5 * time.Second):
		s.T().Fatal("Timeout waiting for message")
	}
}

// TestJSONMessages tests JSON message serialization
func (s *MQTTIntegrationTestSuite) TestJSONMessages() {
	cfg := s.getTestConfig()
	client, err := NewClient(cfg)
	require.NoError(s.T(), err, "Failed to create MQTT client")
	defer client.Close()

	_, err = client.Connect()
	require.NoError(s.T(), err, "Failed to connect to MQTT broker")

	// Test message
	now := time.Now()
	sentMessage := TestMessage{
		ID:        "test-123",
		Content:   "Test JSON Content",
		Value:     123.45,
		Timestamp: now,
		IsActive:  true,
	}

	var receivedMessage TestMessage
	var wg sync.WaitGroup
	wg.Add(1)

	jsonTopic := s.testTopic + "/json"

	// Subscribe to test topic
	err = client.Subscribe(jsonTopic, 0, func(c paho.Client, msg paho.Message) {
		err := json.Unmarshal(msg.Payload(), &receivedMessage)
		require.NoError(s.T(), err, "JSON unmarshaling should succeed")
		wg.Done()
	})
	require.NoError(s.T(), err, "Subscribe should succeed")

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish JSON message
	err = client.WriteJson(jsonTopic, sentMessage)
	require.NoError(s.T(), err, "JSON publish should succeed")

	// Wait for message to be received
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
		// Note: Time comparison might need tolerance due to JSON serialization
	case <-time.After(5 * time.Second):
		s.T().Fatal("Timeout waiting for JSON message")
	}
}

// TestMultipleSubscriptions tests multiple topic subscriptions
func (s *MQTTIntegrationTestSuite) TestMultipleSubscriptions() {
	cfg := s.getTestConfig()
	client, err := NewClient(cfg)
	require.NoError(s.T(), err, "Failed to create MQTT client")
	defer client.Close()

	_, err = client.Connect()
	require.NoError(s.T(), err, "Failed to connect to MQTT broker")

	// Test topics and messages
	topic1 := s.testTopic + "/multi1"
	topic2 := s.testTopic + "/multi2"
	message1 := []byte("Message for topic 1")
	message2 := []byte("Message for topic 2")

	var receivedMessages [][]byte
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	// Handler for multiple topics
	handler := func(c paho.Client, msg paho.Message) {
		mu.Lock()
		receivedMessages = append(receivedMessages, msg.Payload())
		mu.Unlock()
		wg.Done()
	}

	// Subscribe to multiple topics using SubscribeMultiple
	filters := map[string]byte{
		topic1: 0,
		topic2: 0,
	}
	err = client.SubscribeMultiple(filters, handler)
	require.NoError(s.T(), err, "Multiple subscribe should succeed")

	// Give subscriptions time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish to both topics
	err = client.Write(topic1, message1)
	require.NoError(s.T(), err, "Publish to topic1 should succeed")

	err = client.Write(topic2, message2)
	require.NoError(s.T(), err, "Publish to topic2 should succeed")

	// Wait for messages to be received
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		mu.Lock()
		assert.Len(s.T(), receivedMessages, 2, "Should receive 2 messages")

		// Check that we received both messages (order not guaranteed)
		messages := make(map[string]bool)
		for _, msg := range receivedMessages {
			messages[string(msg)] = true
		}
		assert.True(s.T(), messages[string(message1)], "Should receive message1")
		assert.True(s.T(), messages[string(message2)], "Should receive message2")
		mu.Unlock()
	case <-time.After(5 * time.Second):
		s.T().Fatal("Timeout waiting for multiple messages")
	}
}

// TestQoSLevels tests different QoS levels
func (s *MQTTIntegrationTestSuite) TestQoSLevels() {
	for qos := 0; qos <= 2; qos++ {
		s.T().Run(fmt.Sprintf("QoS_%d", qos), func(t *testing.T) {
			cfg := s.getTestConfig()
			cfg.QoS = qos
			client, err := NewClient(cfg)
			require.NoError(t, err, "Failed to create MQTT client")
			defer client.Close()

			_, err = client.Connect()
			require.NoError(t, err, "Failed to connect to MQTT broker")

			qosTopic := fmt.Sprintf("%s/qos%d", s.testTopic, qos)
			testMessage := []byte(fmt.Sprintf("QoS %d test message", qos))
			var receivedMessage []byte
			var wg sync.WaitGroup
			wg.Add(1)

			// Subscribe with specified QoS
			err = client.Subscribe(qosTopic, byte(qos), func(c paho.Client, msg paho.Message) {
				receivedMessage = msg.Payload()
				wg.Done()
			})
			require.NoError(t, err, "Subscribe should succeed")

			// Give subscription time to establish
			time.Sleep(100 * time.Millisecond)

			// Publish message
			err = client.Write(qosTopic, testMessage)
			require.NoError(t, err, "Publish should succeed")

			// Wait for message to be received
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				assert.Equal(t, testMessage, receivedMessage, "Received message should match sent message")
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for QoS message")
			}
		})
	}
}

// TestRetainedMessages tests retained message functionality
func (s *MQTTIntegrationTestSuite) TestRetainedMessages() {
	cfg := s.getTestConfig()
	cfg.Retain = true // Enable retain for this test
	client, err := NewClient(cfg)
	require.NoError(s.T(), err, "Failed to create MQTT client")
	defer client.Close()

	_, err = client.Connect()
	require.NoError(s.T(), err, "Failed to connect to MQTT broker")

	retainTopic := s.testTopic + "/retain"
	retainedMessage := []byte("This is a retained message")

	// Publish retained message first
	err = client.Write(retainTopic, retainedMessage)
	require.NoError(s.T(), err, "Publish retained message should succeed")

	// Give broker time to store the retained message
	time.Sleep(100 * time.Millisecond)

	// Create a new client and subscribe - should receive the retained message
	cfg2 := s.getTestConfig()
	client2, err := NewClient(cfg2)
	require.NoError(s.T(), err, "Failed to create second MQTT client")
	defer client2.Close()

	_, err = client2.Connect()
	require.NoError(s.T(), err, "Failed to connect second client to MQTT broker")

	var receivedMessage []byte
	var wg sync.WaitGroup
	wg.Add(1)

	// Subscribe to the retained topic
	err = client2.Subscribe(retainTopic, 0, func(c paho.Client, msg paho.Message) {
		receivedMessage = msg.Payload()
		wg.Done()
	})
	require.NoError(s.T(), err, "Subscribe should succeed")

	// Wait for retained message to be received
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		assert.Equal(s.T(), retainedMessage, receivedMessage, "Should receive retained message")
	case <-time.After(5 * time.Second):
		s.T().Fatal("Timeout waiting for retained message")
	}
}

// TestChannelSubscribe tests the channel-based subscription functionality
func (s *MQTTIntegrationTestSuite) TestChannelSubscribe() {
	cfg := s.getTestConfig()
	client, err := NewClient(cfg)
	require.NoError(s.T(), err, "Failed to create MQTT client")
	defer client.Close()

	_, err = client.Connect()
	require.NoError(s.T(), err, "Failed to connect to MQTT broker")

	channelTopic := s.testTopic + "/channel"
	testMessage := []byte("Channel subscription test message")

	// Create a buffered channel for messages
	msgChan := make(chan paho.Message, 10)

	// Subscribe using channel
	err = client.ChannelSubscribe(channelTopic, 0, msgChan)
	require.NoError(s.T(), err, "Channel subscribe should succeed")

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Publish message
	err = client.Write(channelTopic, testMessage)
	require.NoError(s.T(), err, "Publish should succeed")

	// Wait for message on channel
	select {
	case msg := <-msgChan:
		assert.Equal(s.T(), testMessage, msg.Payload(), "Channel message should match sent message")
	case <-time.After(5 * time.Second):
		s.T().Fatal("Timeout waiting for channel message")
	}
}

// TestConfigValidation tests configuration validation
func (s *MQTTIntegrationTestSuite) TestConfigValidation() {
	testCases := []struct {
		name        string
		configFunc  func() *Config
		expectError bool
		errorType   error
	}{
		{
			name: "Valid Config",
			configFunc: func() *Config {
				return s.getTestConfig()
			},
			expectError: false,
		},
		{
			name: "Missing Brokers",
			configFunc: func() *Config {
				cfg := s.getTestConfig()
				cfg.Brokers = []string{}
				return cfg
			},
			expectError: true,
			errorType:   ErrMissingBroker,
		},
		{
			name: "Invalid Protocol",
			configFunc: func() *Config {
				cfg := s.getTestConfig()
				cfg.Protocol = "invalid"
				return cfg
			},
			expectError: true,
			errorType:   ErrInvalidProtocol,
		},
		{
			name: "Invalid QoS",
			configFunc: func() *Config {
				cfg := s.getTestConfig()
				cfg.QoS = 5 // QoS must be 0, 1, or 2
				return cfg
			},
			expectError: true,
			errorType:   ErrInvalidQoSLevel,
		},
		{
			name: "Invalid Timeout",
			configFunc: func() *Config {
				cfg := s.getTestConfig()
				cfg.Timeout = -1
				return cfg
			},
			expectError: true,
			errorType:   ErrInvalidTimeout,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			cfg := tc.configFunc()
			err := cfg.Validate()

			if tc.expectError {
				assert.Error(t, err, "Expected validation error")
				if tc.errorType != nil {
					assert.ErrorIs(t, err, tc.errorType, "Error type should match")
				}
			} else {
				assert.NoError(t, err, "Expected no validation error")

				// If validation passes, try to create a client
				client, err := NewClient(cfg)
				assert.NoError(t, err, "Should be able to create client with valid config")
				if client != nil {
					client.Close()
				}
			}
		})
	}
}

// TestNilConfig tests handling of nil configuration
func (s *MQTTIntegrationTestSuite) TestNilConfig() {
	client, err := NewClient(nil)
	assert.Error(s.T(), err, "Should fail with nil config")
	assert.ErrorIs(s.T(), err, ErrNilConfig, "Should return ErrNilConfig")
	assert.Nil(s.T(), client, "Client should be nil")
}

// Run the test suite
func TestMqttIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(MQTTIntegrationTestSuite))
}
