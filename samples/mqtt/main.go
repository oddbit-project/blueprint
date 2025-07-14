package main

import (
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/mqtt"
	"os"
	"time"
)

func main() {
	_ = log.Configure(log.NewDefaultConfig())

	cfg := mqtt.NewConfig()
	cfg.Username = "testUser"
	cfg.Password = "someTestPassword"
	cfg.Protocol = "tcp"
	cfg.Retain = false
	cfg.TLSEnable = true
	cfg.TLSInsecureSkipVerify = true
	cfg.TLSCA = "../../infra/mosquitto/config/certs/ca.crt"
	cfg.TLSCert = "../../infra/mosquitto/config/certs/client.crt"
	cfg.TLSKey = "../../infra/mosquitto/config/certs/client.key"
	cfg.QoS = 0
	cfg.PersistentSession = false
	cfg.Brokers = []string{"localhost:18883"}

	logger := log.New("mqtt-sample")
	client, err := mqtt.NewClient(cfg)
	if err != nil {
		logger.Fatal(err, "cannot initialize mqtt")
		os.Exit(1)
	}
	_, err = client.Connect()
	if err != nil {
		logger.Fatal(err, "cannot connect to mqtt")
		os.Exit(1)
	}
	defer client.Close()

	topicName := "blueprint/test"
	message := []byte("the quick brown fox jumps over the lazy dog")

	var received []byte = nil
	// subscribe topic
	err = client.Subscribe(topicName, 2, func(c paho.Client, msg paho.Message) {
		logger.Infof("Received message: %s", msg.Payload())
		received = msg.Payload()
	})
	if err != nil {
		logger.Fatal(err, "cannot subscribe")
		os.Exit(1)
	}

	// write to topic
	logger.Infof("Writing message: %v", string(message))
	if err = client.Write(topicName, message); err != nil {
		logger.Fatal(err, "cannot publish message")
		os.Exit(1)
	}

	for received == nil {
		// sleep 10ms each time
		time.Sleep(10 * time.Millisecond)
	}
}
