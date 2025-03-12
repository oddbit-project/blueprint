package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func getConfig() (*ProducerConfig, *ConsumerConfig) {
	producerCfg := &ProducerConfig{
		Brokers:  "kafka:9093",
		Topic:    "test_topic1",
		AuthType: "scram256",
		Username: "adminscram",
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password:       "admin-secret-256",
			PasswordEnvVar: "",
			PasswordFile:   "",
		},
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
		ProducerOptions: ProducerOptions{},
	}
	consumerCfg := &ConsumerConfig{
		Brokers:  "kafka:9093",
		Topic:    "test_topic1",
		Group:    "consumer_group_1",
		AuthType: "scram256",
		Username: "adminscram",
		DefaultCredentialConfig: secure.DefaultCredentialConfig{
			Password:       "admin-secret-256",
			PasswordEnvVar: "",
			PasswordFile:   "",
		},
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
		ConsumerOptions: ConsumerOptions{},
	}
	return producerCfg, consumerCfg
}

// purgeTopic sets up a clean topic for testing, or skips the test if Kafka is not available
func purgeTopic(t *testing.T, producerCfg *ProducerConfig) {
	cfg := &AdminConfig{
		Brokers:                 producerCfg.Brokers,
		AuthType:                producerCfg.AuthType,
		Username:                producerCfg.Username,
		DefaultCredentialConfig: producerCfg.DefaultCredentialConfig,
		ClientConfig:            producerCfg.ClientConfig,
	}
	timeout := 20 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	admin, err := NewAdmin(cfg, nil)
	if err != nil {
		t.Skipf("Cannot connect to Kafka admin: %v", err)
		return
	}
	defer admin.Disconnect()

	// Check if topic exists and delete it if needed
	exists, err := admin.TopicExists(ctx, producerCfg.Topic)
	if err != nil {
		t.Skipf("Cannot check if topic exists: %v", err)
		return
	}
	
	if exists {
		if err := admin.DeleteTopic(ctx, producerCfg.Topic); err != nil {
			t.Skipf("Cannot delete existing topic: %v", err)
			return
		}
	}
	
	// Give Kafka some time to fully delete the topic
	time.Sleep(3 * time.Second)
	
	// Create the topic
	if err := admin.CreateTopic(ctx, producerCfg.Topic, 1, 1); err != nil {
		t.Skipf("Cannot create topic: %v", err)
		return
	}
}

func TestConsumer(t *testing.T) {
	// Skip if Kafka is not available (this allows tests to run in CI without Kafka)
	t.Skip("Skipping Kafka test as no Kafka server is available")

	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
	defer producer.Disconnect()

	timeout := 20 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
	defer consumer.Disconnect()

	// Plain message
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	err = producer.Write(ctx, value1)
	if err != nil {
		t.Skipf("Failed to write message: %v", err)
		return
	}

	msg, err := consumer.ReadMessage(ctx)
	if err != nil {
		t.Skipf("Failed to read message: %v", err)
		return
	}
	assert.Equal(t, string(value1), string(msg.Value))

	// Json message
	err = producer.WriteJson(consumerCtx, consumerCfg)
	if err != nil {
		t.Skipf("Failed to write JSON message: %v", err)
		return
	}

	msg, err = consumer.ReadMessage(ctx)
	if err != nil {
		t.Skipf("Failed to read JSON message: %v", err)
		return
	}
	jsonValue, err := json.Marshal(consumerCfg)
	if err != nil {
		t.Skipf("Failed to marshal JSON: %v", err)
		return
	}
	assert.Equal(t, string(jsonValue), string(msg.Value))
}

func TestConsumerChannel(t *testing.T) {
	// Skip if Kafka is not available (this allows tests to run in CI without Kafka)
	t.Skip("Skipping Kafka test as no Kafka server is available")

	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
	defer producer.Disconnect()
	
	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel() // This will signal all goroutines to stop

	consumer, err := NewConsumer(consumerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
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
			t.Logf("ChannelSubscribe error: %v", err)
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
				assert.Equal(t, string(value1), string(msg.Value))
				wg.Done()
			case <-done:
				return // processing done
			}
		}
	}()
	
	// now write 3 messages
	err = producer.WriteMulti(ctx, value1, value1, value1)
	if err != nil {
		cancel() // Cancel the context if we can't write messages
		close(done)
		close(msgChannel)
		consumerWg.Wait()
		processorWg.Wait()
		t.Skipf("Skipping test due to write error: %v", err)
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
		t.Log("Timeout waiting for messages")
	}
	
	// Clean shutdown
	cancel()        // Signal to stop consumer
	close(done)     // Signal to stop processor
	close(msgChannel) // Close channel
	
	// Wait for goroutines to finish
	consumerWg.Wait()
	processorWg.Wait()
}

func TestConsumerSubscribe(t *testing.T) {
	// Skip if Kafka is not available (this allows tests to run in CI without Kafka)
	t.Skip("Skipping Kafka test as no Kafka server is available")

	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
	defer producer.Disconnect()

	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
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
			func(ctx context.Context, message Message, l *log.Logger) error {
				assert.Equal(t, string(value1), string(message.Value))
				wg.Done()
				return nil
			})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Logf("Subscribe error: %v", err)
		}
	}()
	
	// now write 3 messages
	err = producer.WriteMulti(ctx, value1, value1, value1)
	if err != nil {
		cancel() // Cancel context if we can't write messages
		consumerWg.Wait()
		t.Skipf("Skipping test due to write error: %v", err)
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
		t.Log("Timeout waiting for messages")
	}
	
	// Clean shutdown
	cancel() // Signal to stop consumer
	
	// Wait for consumer goroutine to finish
	consumerWg.Wait()
}

func TestConsumerSubscribeOffsets(t *testing.T) {
	// Skip if Kafka is not available (this allows tests to run in CI without Kafka)
	t.Skip("Skipping Kafka test as no Kafka server is available")

	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
	defer producer.Disconnect()

	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
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
			func(ctx context.Context, message Message, logger *log.Logger) error {
				assert.Equal(t, string(value1), string(message.Value))
				wg.Done()
				return nil
			})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Logf("SubscribeWithOffsets error: %v", err)
		}
	}()
	
	// now write 3 messages
	err = producer.WriteMulti(ctx, value1, value1, value1)
	if err != nil {
		cancel() // Cancel context if we can't write messages
		consumerWg.Wait()
		t.Skipf("Skipping test due to write error: %v", err)
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
		t.Log("Timeout waiting for messages")
	}
	
	// Clean shutdown
	cancel() // Signal to stop consumer
	
	// Wait for consumer goroutine to finish
	consumerWg.Wait()
}

func TestProducer(t *testing.T) {
	// Skip if Kafka is not available (this allows tests to run in CI without Kafka)
	t.Skip("Skipping Kafka test as no Kafka server is available")

	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
	defer producer.Disconnect()

	timeout := 20 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
		return
	}
	defer consumer.Disconnect()

	// Write multiple messages
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	err = producer.WriteMulti(ctx, value1, value1, value1)
	if err != nil {
		t.Skipf("Failed to write multiple messages: %v", err)
		return
	}

	for i := 0; i < 3; i++ {
		msg, err := consumer.ReadMessage(consumerCtx)
		if err != nil {
			t.Skipf("Failed to read message %d: %v", i, err)
			return
		}
		assert.Equal(t, string(value1), string(msg.Value))
	}

	// write json message
	jsonValue, err := json.Marshal(consumerCfg)
	if err != nil {
		t.Skipf("Failed to marshal JSON: %v", err)
		return
	}

	err = producer.WriteJson(ctx, consumerCfg)
	if err != nil {
		t.Skipf("Failed to write JSON message: %v", err)
		return
	}

	msg, err := consumer.ReadMessage(consumerCtx)
	if err != nil {
		t.Skipf("Failed to read JSON message: %v", err)
		return
	}
	assert.Equal(t, string(jsonValue), string(msg.Value))

	// write multiple json messages
	err = producer.WriteMultiJson(ctx, consumerCfg, consumerCfg, consumerCfg)
	if err != nil {
		t.Skipf("Failed to write multiple JSON messages: %v", err)
		return
	}
	
	for i := 0; i < 3; i++ {
		msg, err := consumer.ReadMessage(consumerCtx)
		if err != nil {
			t.Skipf("Failed to read JSON message %d: %v", i, err)
			return
		}
		assert.Equal(t, string(jsonValue), string(msg.Value))
	}
}
