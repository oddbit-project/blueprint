package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

// KafkaIntegrationTestSuite manages the Kafka testcontainer and provides configuration
type KafkaIntegrationTestSuite struct {
	suite.Suite
	container     testcontainers.Container
	kafkaInstance *kafka.KafkaContainer
	brokers       string
	ctx           context.Context
}

// SetupSuite sets up the test suite with a shared Kafka container
func (k *KafkaIntegrationTestSuite) SetupSuite() {
	k.ctx = context.Background()

	// Create Kafka container using testcontainers
	var err error
	k.kafkaInstance, err = kafka.Run(k.ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	require.NoError(k.T(), err, "Failed to start Kafka container")

	// Get brokers string
	brokers, err := k.kafkaInstance.Brokers(k.ctx)
	require.NoError(k.T(), err, "Failed to get Kafka brokers")
	k.brokers = strings.Join(brokers, ",")
	k.container = k.kafkaInstance.Container

	k.T().Logf("Kafka container started with brokers: %s", k.brokers)
}

// TearDownSuite cleans up the test suite by stopping the Kafka container
func (k *KafkaIntegrationTestSuite) TearDownSuite() {
	if k.container != nil {
		err := k.container.Terminate(k.ctx)
		if err != nil {
			k.T().Logf("Failed to terminate Kafka container: %v", err)
		}
	}
}

// getConfig returns producer and consumer configurations using the testcontainer brokers
func (k *KafkaIntegrationTestSuite) getConfig() (*ProducerConfig, *ConsumerConfig) {
	producerCfg := &ProducerConfig{
		Brokers:  k.brokers,
		Topic:    "test_topic1",
		AuthType: "none", // Testcontainer doesn't require auth
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
		ProducerOptions: ProducerOptions{},
	}
	consumerCfg := &ConsumerConfig{
		Brokers:  k.brokers,
		Topic:    "test_topic1",
		Group:    "consumer_group_1",
		AuthType: "none", // Testcontainer doesn't require auth
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
		ConsumerOptions: ConsumerOptions{},
	}
	return producerCfg, consumerCfg
}

// purgeTopic sets up a clean topic for testing using the testcontainer
func (k *KafkaIntegrationTestSuite) purgeTopic(producerCfg *ProducerConfig) {
	cfg := &AdminConfig{
		Brokers:      producerCfg.Brokers,
		AuthType:     producerCfg.AuthType,
		ClientConfig: producerCfg.ClientConfig,
	}
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	admin, err := NewAdmin(cfg, nil)
	require.NoError(k.T(), err, "Failed to create Kafka admin")

	// Connect to Kafka admin
	err = admin.Connect(ctx)
	require.NoError(k.T(), err, "Failed to connect to Kafka admin")
	defer admin.Disconnect()

	// Check if topic exists and delete it if needed
	exists, err := admin.TopicExists(ctx, producerCfg.Topic)
	require.NoError(k.T(), err, "Failed to check if topic exists")

	if exists {
		err := admin.DeleteTopic(ctx, producerCfg.Topic)
		require.NoError(k.T(), err, "Failed to delete existing topic")
	}

	// Give Kafka some time to fully delete the topic
	time.Sleep(3 * time.Second)

	// Create the topic
	err = admin.CreateTopic(ctx, producerCfg.Topic, 1, 1)
	require.NoError(k.T(), err, "Failed to create topic")
}

// TestConsumer tests basic producer/consumer operations
func (k *KafkaIntegrationTestSuite) TestConsumer() {
	producerCfg, consumerCfg := k.getConfig()

	// Setup clean topic
	k.purgeTopic(producerCfg)

	producer, err := NewProducer(producerCfg, nil)
	require.NoError(k.T(), err, "Failed to create producer")
	defer producer.Disconnect()

	consumer, err := NewConsumer(consumerCfg, nil)
	require.NoError(k.T(), err, "Failed to create consumer")
	defer consumer.Disconnect()

	// Plain message
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	err = producer.Write(k.ctx, value1)
	require.NoError(k.T(), err, "Failed to write message")

	msg, err := consumer.ReadMessage(k.ctx)
	require.NoError(k.T(), err, "Failed to read message")
	assert.Equal(k.T(), string(value1), string(msg.Value))

	// Json message
	err = producer.WriteJson(k.ctx, consumerCfg)
	require.NoError(k.T(), err, "Failed to write JSON message")

	msg, err = consumer.ReadMessage(k.ctx)
	require.NoError(k.T(), err, "Failed to read JSON message")
	jsonValue, err := json.Marshal(consumerCfg)
	require.NoError(k.T(), err, "Failed to marshal JSON")
	assert.Equal(k.T(), string(jsonValue), string(msg.Value))
}

// TestConsumerChannel tests channel-based message processing
func (k *KafkaIntegrationTestSuite) TestConsumerChannel() {
	producerCfg, consumerCfg := k.getConfig()

	// Setup clean topic
	k.purgeTopic(producerCfg)

	producer, err := NewProducer(producerCfg, nil)
	require.NoError(k.T(), err, "Failed to create producer")
	defer producer.Disconnect()

	timeout := 10 * time.Second
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel() // This will signal all goroutines to stop

	consumer, err := NewConsumer(consumerCfg, nil)
	require.NoError(k.T(), err, "Failed to create consumer")
	defer consumer.Disconnect()

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message

	// Create a done channel to signal when we're finished processing
	done := make(chan struct{})
	msgChannel := make(chan Message)

	// consumer thread - make sure it stops when test ends
	consumerWg := sync.WaitGroup{}
	consumerWg.Add(1)
	go func() {
		defer consumerWg.Done()
		err := consumer.ChannelSubscribe(consumerCtx, msgChannel)
		if err != nil && !errors.Is(err, context.Canceled) {
			k.T().Logf("ChannelSubscribe error: %v", err)
		}
	}()

	// channel process thread
	processorWg := sync.WaitGroup{}
	processorWg.Add(1)
	go func() {
		defer processorWg.Done()
		for {
			select {
			case msg, ok := <-msgChannel:
				if !ok {
					return // channel closed
				}
				assert.Equal(k.T(), string(value1), string(msg.Value))
				wg.Done()
			case <-done:
				return // processing done
			}
		}
	}()

	// now write 3 messages
	err = producer.WriteMulti(k.ctx, value1, value1, value1)
	if err != nil {
		cancel() // Cancel the context if we can't write messages
		close(done)
		close(msgChannel)
		consumerWg.Wait()
		processorWg.Wait()
		require.NoError(k.T(), err, "Failed to write multiple messages")
		return
	}

	// Wait for all messages to be processed or timeout
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// Success! All messages processed
	case <-time.After(timeout):
		k.T().Log("Timeout waiting for messages")
	}

	// Clean shutdown
	cancel()          // Signal to stop consumer
	close(done)       // Signal to stop processor
	close(msgChannel) // Close channel

	// Wait for goroutines to finish
	consumerWg.Wait()
	processorWg.Wait()
}

// TestConsumerSubscribe tests subscription-based consumption
func (k *KafkaIntegrationTestSuite) TestConsumerSubscribe() {
	producerCfg, consumerCfg := k.getConfig()

	// Setup clean topic
	k.purgeTopic(producerCfg)

	producer, err := NewProducer(producerCfg, nil)
	require.NoError(k.T(), err, "Failed to create producer")
	defer producer.Disconnect()

	timeout := 10 * time.Second
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	require.NoError(k.T(), err, "Failed to create consumer")
	defer consumer.Disconnect()

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message

	// consumer thread with proper cleanup
	consumerWg := sync.WaitGroup{}
	consumerWg.Add(1)
	go func() {
		defer consumerWg.Done()
		err := consumer.Subscribe(consumerCtx,
			func(ctx context.Context, message Message) error {
				assert.Equal(k.T(), string(value1), string(message.Value))
				wg.Done()
				return nil
			})
		if err != nil && !errors.Is(err, context.Canceled) {
			k.T().Logf("Subscribe error: %v", err)
		}
	}()

	// now write 3 messages
	err = producer.WriteMulti(k.ctx, value1, value1, value1)
	require.NoError(k.T(), err, "Failed to write multiple messages")

	// Wait for all messages to be processed or timeout
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// Success! All messages processed
	case <-time.After(timeout):
		k.T().Log("Timeout waiting for messages")
	}

	// Clean shutdown
	cancel() // Signal to stop consumer

	// Wait for consumer goroutine to finish
	consumerWg.Wait()
}

// TestConsumerSubscribeOffsets tests offset management
func (k *KafkaIntegrationTestSuite) TestConsumerSubscribeOffsets() {
	producerCfg, consumerCfg := k.getConfig()

	// Setup clean topic
	k.purgeTopic(producerCfg)

	producer, err := NewProducer(producerCfg, nil)
	require.NoError(k.T(), err, "Failed to create producer")
	defer producer.Disconnect()

	timeout := 10 * time.Second
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	require.NoError(k.T(), err, "Failed to create consumer")
	defer consumer.Disconnect()

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message

	// consumer thread with proper cleanup
	consumerWg := sync.WaitGroup{}
	consumerWg.Add(1)
	go func() {
		defer consumerWg.Done()
		err := consumer.SubscribeWithOffsets(
			consumerCtx,
			func(ctx context.Context, message Message) error {
				assert.Equal(k.T(), string(value1), string(message.Value))
				wg.Done()
				return nil
			})
		if err != nil && !errors.Is(err, context.Canceled) {
			k.T().Logf("SubscribeWithOffsets error: %v", err)
		}
	}()

	// now write 3 messages
	err = producer.WriteMulti(k.ctx, value1, value1, value1)
	require.NoError(k.T(), err, "Failed to write multiple messages")

	// Wait for all messages to be processed or timeout
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// Success! All messages processed
	case <-time.After(timeout):
		k.T().Log("Timeout waiting for messages")
	}

	// Clean shutdown
	cancel() // Signal to stop consumer

	// Wait for consumer goroutine to finish
	consumerWg.Wait()
}

// TestProducer tests producer functionality
func (k *KafkaIntegrationTestSuite) TestProducer() {
	producerCfg, consumerCfg := k.getConfig()

	// Setup clean topic
	k.purgeTopic(producerCfg)

	producer, err := NewProducer(producerCfg, nil)
	require.NoError(k.T(), err, "Failed to create producer")
	defer producer.Disconnect()

	timeout := 10 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	require.NoError(k.T(), err, "Failed to create consumer")
	defer consumer.Disconnect()

	// Write multiple messages
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	err = producer.WriteMulti(k.ctx, value1, value1, value1)
	require.NoError(k.T(), err, "Failed to write multiple messages")

	for i := 0; i < 3; i++ {
		msg, err := consumer.ReadMessage(consumerCtx)
		require.NoError(k.T(), err, fmt.Sprintf("Failed to read message %d", i))
		assert.Equal(k.T(), string(value1), string(msg.Value))
	}

	// write json message
	jsonValue, err := json.Marshal(consumerCfg)
	require.NoError(k.T(), err, "Failed to marshal JSON")

	err = producer.WriteJson(k.ctx, consumerCfg)
	require.NoError(k.T(), err, "Failed to write JSON message")

	msg, err := consumer.ReadMessage(consumerCtx)
	require.NoError(k.T(), err, "Failed to read JSON message")
	assert.Equal(k.T(), string(jsonValue), string(msg.Value))

	// write multiple json messages
	err = producer.WriteMultiJson(k.ctx, consumerCfg, consumerCfg, consumerCfg)
	require.NoError(k.T(), err, "Failed to write multiple JSON messages")

	for i := 0; i < 3; i++ {
		msg, err := consumer.ReadMessage(consumerCtx)
		require.NoError(k.T(), err, fmt.Sprintf("Failed to read JSON message %d", i))
		assert.Equal(k.T(), string(jsonValue), string(msg.Value))
	}
}

// TestKafkaIntegration runs the integration test suite
func TestKafkaIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(KafkaIntegrationTestSuite))
}
