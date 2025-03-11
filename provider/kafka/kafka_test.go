package kafka

import (
	"context"
	"encoding/json"
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
	assert.Nil(t, err)
	if exists, err := admin.TopicExists(ctx, producerCfg.Topic); err != nil {
		t.Error(err)
	} else {
		if exists {
			assert.Nil(t, admin.DeleteTopic(ctx, producerCfg.Topic))
		}
	}
	time.Sleep(3 * time.Second) // settling time
	assert.Nil(t, admin.CreateTopic(ctx, producerCfg.Topic, 1, 1))
	admin.Disconnect()
}

func TestConsumer(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 20 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// Plain message
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	err = producer.Write(ctx, value1)
	assert.Nil(t, err)

	msg, err := consumer.ReadMessage(ctx)
	assert.Nil(t, err)
	assert.Equal(t, string(value1), string(msg.Value))

	// Json message
	err = producer.WriteJson(consumerCtx, consumerCfg)
	assert.Nil(t, err)

	msg, err = consumer.ReadMessage(ctx)
	assert.Nil(t, err)
	jsonValue, err := json.Marshal(consumerCfg)
	assert.Nil(t, err)
	assert.Equal(t, string(jsonValue), string(msg.Value))

}

func TestConsumerChannel(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message
	msgChannel := make(chan Message)
	defer close(msgChannel)
	// consumer thread
	go func() {
		assert.NoError(t, consumer.ChannelSubscribe(consumerCtx, msgChannel))
	}()
	// channel process thread
	go func() {
		for msg := range msgChannel {
			assert.Equal(t, string(value1), string(msg.Value))
			wg.Done()
		}
	}()
	// now write 3 messages
	err = producer.WriteMulti(ctx, value1, value1, value1)
	assert.Nil(t, err)

	// wait for conclusion
	wg.Wait()
}

func TestConsumerSubscribe(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message
	// consumer thread
	go func() {
		assert.NoError(t,
			consumer.Subscribe(consumerCtx,
				func(ctx context.Context, message Message, l *log.Logger) error {
					assert.Equal(t, string(value1), string(message.Value))
					wg.Done()
					return nil
				}))
	}()
	// now write 3 messages
	err = producer.WriteMulti(ctx, value1, value1, value1)
	assert.Nil(t, err)

	// wait for conclusion
	wg.Wait()
}

func TestConsumerSubscribeOffsets(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message
	// consumer thread
	go func() {
		assert.NoError(t,
			consumer.SubscribeWithOffsets(
				consumerCtx,
				func(ctx context.Context, message Message, logger *log.Logger) error {
					assert.Equal(t, string(value1), string(message.Value))
					wg.Done()
					return nil
				}))
	}()
	// now write 3 messages
	err = producer.WriteMulti(ctx, value1, value1, value1)
	assert.Nil(t, err)

	// wait for conclusion
	wg.Wait()
}

func TestProducer(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	purgeTopic(t, producerCfg)
	producer, err := NewProducer(producerCfg, nil)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 20 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCfg, nil)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// Write multiple messages
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	err = producer.WriteMulti(ctx, value1, value1, value1)
	assert.Nil(t, err)

	for i := 0; i < 3; i++ {
		msg, err := consumer.ReadMessage(consumerCtx)
		assert.Nil(t, err)
		assert.Equal(t, string(value1), string(msg.Value))
	}

	// write json message
	jsonValue, err := json.Marshal(consumerCfg)
	assert.Nil(t, err)

	err = producer.WriteJson(ctx, consumerCfg)
	assert.Nil(t, err)

	msg, err := consumer.ReadMessage(consumerCtx)
	assert.Nil(t, err)
	assert.Equal(t, string(jsonValue), string(msg.Value))

	// write multiple json messages
	err = producer.WriteMultiJson(ctx, consumerCfg, consumerCfg, consumerCfg)
	assert.Nil(t, err)
	for i := 0; i < 3; i++ {
		msg, err := consumer.ReadMessage(consumerCtx)
		assert.Nil(t, err)
		assert.Equal(t, string(jsonValue), string(msg.Value))
	}

}
