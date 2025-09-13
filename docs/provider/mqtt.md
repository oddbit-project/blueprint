# blueprint.provider.mqtt

Blueprint MQTT client

## Configuration

The MQTT client uses the following configuration:

```json
{
  "mqtt": {
    "brokers": ["127.0.0.1:1883"],
    "protocol": "tcp",
    "username": "",
    "password": "",
    "timeout": 5,
    "connectionTimeout": 30,
    "qos": 0,
    "clientId": "",
    "retain": false,
    "keepAlive": 0,
    "autoReconnect": true,
    "persistentSession": false,
    "tlsEnable": false,
    "tlsCa": "",
    "tlsCert": "",
    "tlsKey": "",
    "tlsKeyPassword": "",
    "tlsInsecureVerify": true
  }
}
```

# Using the MQTT client

```go
package main

import (
 paho "github.com/eclipse/paho.mqtt.golang"
 "github.com/oddbit-project/blueprint/log/zerolog/writer"
 "github.com/oddbit-project/blueprint/provider/mqtt"
 "github.com/rs/zerolog/log"
 "time"
)

func main() {
	// use zerolog as logger with console writer
	writer.UseDefaultWriter()

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

	client, err := mqtt.NewClient(cfg)
	if err != nil {
		log.Fatal().Msgf("cannot initialize mqtt: %v", err)
	}
	_, err = client.Connect()
	if err != nil {
		log.Fatal().Msgf("cannot connect to mqtt: %v", err)
	}
	defer client.Close()

	topicName := "blueprint/test"
	message := []byte("the quick brown fox jumps over the lazy dog")

	var received []byte = nil
	// subscribe topic
	client.Subscribe(topicName, 2, func(c paho.Client, msg paho.Message) {
		log.Info().Msgf("Received message: %s", msg.Payload())
		received = msg.Payload()
	})

	// write to topic
	log.Info().Msgf("Writing message: %v", string(message))
	client.Write(topicName, message)

	for received == nil {
		// sleep 10ms each time
		time.Sleep(10 * time.Millisecond)
	}
}
```
