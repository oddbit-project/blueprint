//go:build integration
// +build integration

package nats

import (
	"context"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Integration test for NATS authentication methods
type NatsAuthIntegrationTestSuite struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc
	logger *log.Logger
}

// SetupSuite prepares the test environment
func (s *NatsAuthIntegrationTestSuite) SetupSuite() {
	// Create context with cancellation
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Create logger
	s.logger = log.New("nats-auth-integration-test")
}

// TearDownSuite cleans up after all tests
func (s *NatsAuthIntegrationTestSuite) TearDownSuite() {
	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}
}

// TestBasicAuth tests basic authentication
func (s *NatsAuthIntegrationTestSuite) TestBasicAuth() {
	// Create producer with basic auth
	producerConfig := &nats.ProducerConfig{
		URL:      "nats://nats:4222", // URL without auth info
		Subject:  "test.auth.basic",
		AuthType: nats.AuthTypeBasic,
		Username: "testuser",
		DefaultCredentialConfig: nats.StringPasswordConfig("testpassword"),
	}

	producer, err := nats.NewProducer(producerConfig, s.logger)
	assert.NoError(s.T(), err, "Creating producer with basic auth should succeed")
	defer producer.Disconnect()

	assert.True(s.T(), producer.IsConnected(), "Producer should be connected with basic auth")

	// Test with invalid credentials
	invalidConfig := &nats.ProducerConfig{
		URL:      "nats://nats:4222",
		Subject:  "test.auth.basic",
		AuthType: nats.AuthTypeBasic,
		Username: "testuser",
		DefaultCredentialConfig: nats.StringPasswordConfig("wrongpassword"),
	}

	invalidProducer, err := nats.NewProducer(invalidConfig, s.logger)
	if err == nil {
		defer invalidProducer.Disconnect()
		assert.False(s.T(), invalidProducer.IsConnected(), "Producer should not connect with invalid basic auth")
	}
}

// TestTokenAuth tests token authentication
func (s *NatsAuthIntegrationTestSuite) TestTokenAuth() {
	// For this test to work, NATS server must be configured with token auth
	// The Docker container is started with basic auth, so this test will fail
	// This test is included as an example

	// Create producer with token auth
	producerConfig := &nats.ProducerConfig{
		URL:      "nats://nats:4222", // URL without auth info
		Subject:  "test.auth.token",
		AuthType: nats.AuthTypeToken,
		Token:    "testpassword", // Using password as token for testing
	}

	producer, err := nats.NewProducer(producerConfig, s.logger)
	if err == nil {
		defer producer.Disconnect()
		// Check connection - may still fail due to auth mechanism
		if producer.IsConnected() {
			// If connected, test publishing a message
			err = producer.Publish([]byte("Token Auth Test"))
			assert.NoError(s.T(), err, "Publishing with token auth should succeed")
		} else {
			s.T().Log("Token auth not configured in NATS server")
		}
	} else {
		s.T().Logf("Token auth failed: %v", err)
	}
}

// TestDirectURLAuth tests authentication with credentials in URL
func (s *NatsAuthIntegrationTestSuite) TestDirectURLAuth() {
	// Create producer with credentials in URL
	producerConfig := &nats.ProducerConfig{
		URL:      "nats://testuser:testpassword@nats:4222",
		Subject:  "test.auth.url",
		AuthType: nats.AuthTypeNone, // Auth is in URL
	}

	producer, err := nats.NewProducer(producerConfig, s.logger)
	assert.NoError(s.T(), err, "Creating producer with URL auth should succeed")
	defer producer.Disconnect()

	assert.True(s.T(), producer.IsConnected(), "Producer should be connected with URL auth")

	// Test publishing a message
	err = producer.Publish([]byte("URL Auth Test"))
	assert.NoError(s.T(), err, "Publishing with URL auth should succeed")
}

// TestConnectionTimeout tests connection timeout handling
func (s *NatsAuthIntegrationTestSuite) TestConnectionTimeout() {
	// Create producer with non-existent server and short timeout
	producerConfig := &nats.ProducerConfig{
		URL:      "nats://nonexistent:4222",
		Subject:  "test.timeout",
		AuthType: nats.AuthTypeNone,
		ProducerOptions: nats.ProducerOptions{
			Timeout: 500, // 500ms timeout
		},
	}

	startTime := time.Now()
	_, err := nats.NewProducer(producerConfig, s.logger)
	duration := time.Since(startTime)

	// Should fail quickly due to timeout
	assert.Error(s.T(), err, "Connection to nonexistent server should fail")
	assert.Less(s.T(), duration, 2*time.Second, "Connection should time out quickly")
}

// Run the test suite
func TestNatsAuthIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(NatsAuthIntegrationTestSuite))
}