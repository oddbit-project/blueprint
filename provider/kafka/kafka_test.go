package kafka

import (
	"context"
	"encoding/json"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func getConfig() (ProducerConfig, ConsumerConfig) {
	producerCfg := ProducerConfig{
		Brokers:  "kafka:9093",
		Topic:    "test_topic1",
		AuthType: "scram256",
		Username: "adminscram",
		Password: "admin-secret-256",
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
	}
	consumerCfg := ConsumerConfig{
		Brokers:  "kafka:9093",
		Topic:    "test_topic1",
		Group:    "consumer_group_1",
		AuthType: "scram256",
		Username: "adminscram",
		Password: "admin-secret-256",
		ClientConfig: tlsProvider.ClientConfig{
			TLSEnable: false,
		},
	}
	return producerCfg, consumerCfg
}

func purgeTopic(t *testing.T, producerCfg ProducerConfig) {
	cfg := AdminConfig{
		Brokers:      producerCfg.Brokers,
		AuthType:     producerCfg.AuthType,
		Username:     producerCfg.Username,
		Password:     producerCfg.Password,
		ClientConfig: producerCfg.ClientConfig,
	}
	timeout := 20 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	admin, err := NewAdmin(ctx, cfg)
	assert.Nil(t, err)
	if exists, err := admin.TopicExists(producerCfg.Topic); err != nil {
		t.Error(err)
	} else {
		if exists {
			assert.Nil(t, admin.DeleteTopic(producerCfg.Topic))
		}
	}
	time.Sleep(3 * time.Second) // settling time
	assert.Nil(t, admin.CreateTopic(producerCfg.Topic, 1, 1))
	admin.Disconnect()
}

func TestConsumer(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(ctx, producerCfg)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 20 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCtx, consumerCfg)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// Plain message
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	err = producer.Write(value1)
	assert.Nil(t, err)

	msg, err := consumer.ReadMessage()
	assert.Nil(t, err)
	assert.Equal(t, string(value1), string(msg.Value))

	// Json message
	err = producer.WriteJson(consumerCfg)
	assert.Nil(t, err)

	msg, err = consumer.ReadMessage()
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
	producer, err := NewProducer(ctx, producerCfg)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCtx, consumerCfg)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message
	msgChannel := make(chan kafka.Message)
	defer close(msgChannel)
	// consumer thread
	go func() {
		consumer.ChannelSubscribe(msgChannel)
	}()
	// channel process thread
	go func() {
		for msg := range msgChannel {
			assert.Equal(t, string(value1), string(msg.Value))
			wg.Done()
		}
	}()
	// now write 3 messages
	err = producer.WriteMulti(value1, value1, value1)
	assert.Nil(t, err)

	// wait for conclusion
	wg.Wait()
}

func TestConsumerSubscribe(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(ctx, producerCfg)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCtx, consumerCfg)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message
	// consumer thread
	go func() {
		consumer.Subscribe(func(ctx context.Context, message kafka.Message) error {
			assert.Equal(t, string(value1), string(message.Value))
			wg.Done()
			return nil
		})
	}()
	// now write 3 messages
	err = producer.WriteMulti(value1, value1, value1)
	assert.Nil(t, err)

	// wait for conclusion
	wg.Wait()
}

func TestConsumerSubscribeOffsets(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	// remove Topic if exists
	purgeTopic(t, producerCfg)
	producer, err := NewProducer(ctx, producerCfg)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 30 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCtx, consumerCfg)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// consume channel
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(3) // expect 3 message
	// consumer thread
	go func() {
		consumer.SubscribeWithOffsets(func(ctx context.Context, message kafka.Message) error {
			assert.Equal(t, string(value1), string(message.Value))
			wg.Done()
			return nil
		})
	}()
	// now write 3 messages
	err = producer.WriteMulti(value1, value1, value1)
	assert.Nil(t, err)

	// wait for conclusion
	wg.Wait()
}

func TestProducer(t *testing.T) {
	ctx := context.Background()
	producerCfg, consumerCfg := getConfig()

	purgeTopic(t, producerCfg)
	producer, err := NewProducer(ctx, producerCfg)
	defer producer.Disconnect()
	assert.Nil(t, err)

	timeout := 20 * time.Second
	consumerCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consumer, err := NewConsumer(consumerCtx, consumerCfg)
	defer consumer.Disconnect()
	assert.Nil(t, err)

	// Write multiple messages
	value1 := []byte("the quick brown fox jumps over the lazy dog")
	err = producer.WriteMulti(value1, value1, value1)
	assert.Nil(t, err)

	for i := 0; i < 3; i++ {
		msg, err := consumer.ReadMessage()
		assert.Nil(t, err)
		assert.Equal(t, string(value1), string(msg.Value))
	}

	// write json message
	jsonValue, err := json.Marshal(consumerCfg)
	assert.Nil(t, err)

	err = producer.WriteJson(consumerCfg)
	assert.Nil(t, err)

	msg, err := consumer.ReadMessage()
	assert.Nil(t, err)
	assert.Equal(t, string(jsonValue), string(msg.Value))

	// write multiple json messages
	err = producer.WriteMultiJson(consumerCfg, consumerCfg, consumerCfg)
	assert.Nil(t, err)
	for i := 0; i < 3; i++ {
		msg, err := consumer.ReadMessage()
		assert.Nil(t, err)
		assert.Equal(t, string(jsonValue), string(msg.Value))
	}

}
