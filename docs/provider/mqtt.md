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

## Additional Methods

### Publishing

```go
// Write publishes a message to a topic
func (c *Client) Write(topic string, value []byte) error

// WriteJson publishes a JSON-encoded message
func (c *Client) WriteJson(topic string, value interface{}) error
```

**Example:**
```go
// Publish raw bytes
client.Write("sensors/temperature", []byte("25.5"))

// Publish JSON
type SensorData struct {
    DeviceID    string  `json:"device_id"`
    Temperature float64 `json:"temperature"`
    Timestamp   int64   `json:"timestamp"`
}
data := SensorData{DeviceID: "sensor-1", Temperature: 25.5, Timestamp: time.Now().Unix()}
client.WriteJson("sensors/data", data)
```

### Subscribing

```go
// Subscribe subscribes to a topic with a message handler
func (c *Client) Subscribe(topic string, qos byte, handler paho.MessageHandler) error

// SubscribeMultiple subscribes to multiple topics with different QoS levels
func (c *Client) SubscribeMultiple(filters map[string]byte, handler paho.MessageHandler) error

// ChannelSubscribe subscribes to a topic and sends messages to a channel
func (c *Client) ChannelSubscribe(topic string, qos byte, ch chan paho.Message) error

// BufferedChannelSubscribe subscribes with a buffered channel
func (c *Client) BufferedChannelSubscribe(topic string, qos byte, ch chan paho.Message, bufferSize int) error
```

**Example using channel-based subscription:**
```go
msgChan := make(chan paho.Message, 100)

// Subscribe using channel
if err := client.ChannelSubscribe("sensors/#", 1, msgChan); err != nil {
    log.Fatal(err)
}

// Process messages from channel
go func() {
    for msg := range msgChan {
        log.Printf("Topic: %s, Payload: %s", msg.Topic(), string(msg.Payload()))
    }
}()
```

### Routing

```go
// AddRoute adds a topic-specific message handler
func (c *Client) AddRoute(topic string, handler paho.MessageHandler)
```

**Example:**
```go
// Add routes for different topics
client.AddRoute("sensors/temperature", func(c paho.Client, msg paho.Message) {
    log.Printf("Temperature: %s", string(msg.Payload()))
})

client.AddRoute("sensors/humidity", func(c paho.Client, msg paho.Message) {
    log.Printf("Humidity: %s", string(msg.Payload()))
})
```

### Connection Management

```go
func (c *Client) Connect() (paho.Token, error)  // Connect to broker
func (c *Client) Close()                         // Disconnect from broker
func (c *Client) IsConnected() bool             // Check connection status
```

## Configuration Options

The full configuration structure:

```go
type Config struct {
    Brokers              []string `json:"brokers"`              // Broker addresses
    Protocol             string   `json:"protocol"`             // Protocol (tcp, ssl, ws, wss)
    Username             string   `json:"username"`             // MQTT username
    Password             string   `json:"password"`             // MQTT password
    Timeout              int64    `json:"timeout"`              // Operation timeout in seconds
    ConnectionTimeout    int64    `json:"connectionTimeout"`    // Connection timeout in seconds
    QoS                  byte     `json:"qos"`                  // Quality of Service (0, 1, 2)
    ClientId             string   `json:"clientId"`             // Client identifier
    Retain               bool     `json:"retain"`               // Retain messages
    KeepAlive            int64    `json:"keepAlive"`            // Keep-alive interval in seconds
    AutoReconnect        bool     `json:"autoReconnect"`        // Auto-reconnect on disconnect
    PersistentSession    bool     `json:"persistentSession"`    // Maintain session across reconnects
    TLSEnable            bool     `json:"tlsEnable"`            // Enable TLS
    TLSCA                string   `json:"tlsCa"`                // CA certificate path
    TLSCert              string   `json:"tlsCert"`              // Client certificate path
    TLSKey               string   `json:"tlsKey"`               // Client key path
    TLSKeyPassword       string   `json:"tlsKeyPassword"`       // Key password
    TLSInsecureSkipVerify bool    `json:"tlsInsecureVerify"`   // Skip certificate verification
}
```

### Event Handlers

The client supports custom handlers for connection events:

```go
type MqttHandlers struct {
    OnConnect         paho.OnConnectHandler
    OnConnectionLost  paho.ConnectionLostHandler
    OnReconnecting    paho.ReconnectHandler
}

// Create client with custom handlers
handlers := &mqtt.MqttHandlers{
    OnConnect: func(c paho.Client) {
        log.Println("Connected to MQTT broker")
    },
    OnConnectionLost: func(c paho.Client, err error) {
        log.Printf("Connection lost: %v", err)
    },
}
client, err := mqtt.NewClientWithHandlers(cfg, handlers)
```
