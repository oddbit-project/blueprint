package mqtt

import (
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

const mqttUser = "testUser"
const mqttPassword = "someTestPassword"
const (
	mqttCA          = "../../test-deps/mosquitto/config/certs/ca.crt"
	mqttCert        = "../../test-deps/mosquitto/config/certs/client.crt"
	mqttKey         = "../../test-deps/mosquitto/config/certs/client.key"
	mqttPlainBroker = "mosquitto:1883"
	mqttTLSBroker   = "mosquitto:8883"
	mqttTopic       = "blueprint/test"
)

func getPlainConfig() *Config {
	cfg := NewConfig()
	cfg.Username = mqttUser
	cfg.Password = mqttPassword
	cfg.Protocol = "tcp"
	cfg.Retain = false
	cfg.QoS = 0
	cfg.PersistentSession = false
	cfg.Brokers = []string{mqttPlainBroker}
	return cfg
}

func getTLSconfig() *Config {
	cfg := NewConfig()
	cfg.Username = mqttUser
	cfg.Password = mqttPassword
	cfg.Protocol = "tcp"
	cfg.Retain = false
	cfg.TLSEnable = false
	cfg.TLSInsecureSkipVerify = true
	cfg.QoS = 0
	cfg.PersistentSession = false
	cfg.Brokers = []string{mqttTLSBroker}
	cfg.TLSEnable = true
	cfg.TLSCA = mqttCA
	cfg.TLSCert = mqttCert
	cfg.TLSKey = mqttKey
	return cfg
}

func TestPlainClient(t *testing.T) {
	cfg := getTLSconfig()
	client, err := NewClient(cfg)
	assert.Nil(t, err)

	_, err = client.Connect()
	assert.Nil(t, err)

	defer client.Close()

	message := []byte("the quick brown fox jumps over the lazy dog")
	wg := sync.WaitGroup{}
	wg.Add(1)

	// subscribe topic and wait for message
	// subscribe is non-blocking
	client.Subscribe(mqttTopic, 2, func(c paho.Client, msg paho.Message) {
		received := msg.Payload()
		assert.Equal(t, message, received)
		wg.Done()
	})

	// write message to topic
	assert.Nil(t, client.Write(mqttTopic, message))
	wg.Wait()
}
