package mqtt

import (
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/oddbit-project/blueprint/generator"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	DefaultTimeout           = 5  // seconds
	DefaultConnectionTimeout = 30 // seconds

	ErrMissingBroker   = utils.Error("at least one broker must be specified")
	ErrInvalidProtocol = utils.Error("invalid protocol")
	ErrInvalidTimeout  = utils.Error("invalid timeout")
	ErrInvalidQoSLevel = utils.Error("invalid QoS level")
	ErrPublishTimeout  = utils.Error("timeout when publishing")
)

type MqttHandlers struct {
	DefaultPublishHandler  paho.MessageHandler
	OnConnect              paho.OnConnectHandler
	OnConnectionLost       paho.ConnectionLostHandler
	OnReconnecting         paho.ReconnectHandler
	OnConnectAttempt       paho.ConnectionAttemptHandler
	CustomOpenConnectionFn paho.OpenConnectionFunc
}

type Config struct {
	Brokers           []string `json:"brokers"`
	Protocol          string   `json:"protocol"`
	Username          string   `json:"username"`
	Password          string   `json:"password"`
	Timeout           int      `json:"timeout"`
	ConnectionTimeout int      `json:"connectionTimeout"`
	QoS               int      `json:"qos"`
	ClientID          string   `json:"clientId"`
	Retain            bool     `json:"retain"`
	KeepAlive         int64    `json:"keepAlive"`
	AutoReconnect     bool     `json:"autoReconnect"`
	PersistentSession bool     `json:"persistentSession"`
	tlsProvider.ClientConfig
	MqttHandlers `json:"-"`
}

type Client struct {
	ClientOptions *paho.ClientOptions
	Client        paho.Client
	QoS           byte
	Timeout       time.Duration
	Retain        bool
}

type connectToken interface {
	SessionPresent() bool
	ReturnCode() byte
}

func NewConfig() *Config {
	return &Config{
		Brokers:           nil,
		Protocol:          "tcp",
		Username:          "",
		Password:          "",
		Timeout:           DefaultTimeout,
		ConnectionTimeout: DefaultConnectionTimeout,
		QoS:               0,
		ClientID:          "",
		Retain:            false,
		KeepAlive:         0,
		AutoReconnect:     true,
		PersistentSession: false,
		ClientConfig: tlsProvider.ClientConfig{
			TLSCA:                 "",
			TLSCert:               "",
			TLSKey:                "",
			TLSKeyPwd:             "",
			TLSEnable:             false,
			TLSInsecureSkipVerify: false,
		},
		MqttHandlers: MqttHandlers{
			DefaultPublishHandler:  nil,
			OnConnect:              nil,
			OnConnectionLost:       nil,
			OnReconnecting:         nil,
			OnConnectAttempt:       nil,
			CustomOpenConnectionFn: nil,
		},
	}
}

func (c *Config) Validate() error {

	if len(c.Brokers) == 0 {
		return ErrMissingBroker
	}
	if c.Protocol != "tcp" && c.Protocol != "ssl" {
		return ErrInvalidProtocol
	}
	if c.Timeout < 0 {
		return ErrInvalidTimeout
	}
	if c.ConnectionTimeout < 0 {
		return ErrInvalidTimeout
	}
	if c.QoS < 0 || c.QoS > 2 {
		return ErrInvalidQoSLevel
	}
	if c.KeepAlive < 0 {
		return fmt.Errorf("keep alive must be greater than zero")
	}
	return nil
}

func NewClient(cfg *Config) (*Client, error) {
	var opts *paho.ClientOptions
	var err error
	if opts, err = clientOptions(cfg); err != nil {
		return nil, err
	}
	result := &Client{
		ClientOptions: opts,
		Client:        nil,
		QoS:           byte(cfg.QoS),
		Timeout:       time.Duration(cfg.Timeout) * time.Second,
		Retain:        true,
	}

	// run extra configurations
	if err = result.CustomSettings(); err != nil {
		return nil, err
	}

	// create client
	result.Client = paho.NewClient(opts)
	return result, nil
}

func clientOptions(cfg *Config) (*paho.ClientOptions, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	opts := paho.NewClientOptions()
	opts.KeepAlive = cfg.KeepAlive
	opts.WriteTimeout = time.Duration(cfg.Timeout) * time.Second
	opts.ConnectTimeout = time.Duration(cfg.ConnectionTimeout) * time.Second

	if cfg.ClientID == "" {
		cfg.ClientID = generator.RandomString(12)
	}
	opts.SetClientID(cfg.ClientID)

	if cfg.TLSEnable {
		cfg.Protocol = "ssl"
	}

	if cfg.TLSEnable {
		tlsCfg, err := cfg.ClientConfig.TLSConfig()
		if err != nil {
			return nil, err
		}
		opts.SetTLSConfig(tlsCfg)
	}
	opts.SetCleanSession(!cfg.PersistentSession)
	opts.SetAutoReconnect(cfg.AutoReconnect)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)

	// broker addresses
	for _, broker := range cfg.Brokers {
		brokerURI := fmt.Sprintf("%s://%s", cfg.Protocol, broker)
		opts.AddBroker(brokerURI)
	}

	// custom handlers
	if cfg.MqttHandlers.DefaultPublishHandler != nil {
		opts.DefaultPublishHandler = cfg.MqttHandlers.DefaultPublishHandler
	}
	if cfg.MqttHandlers.OnConnect != nil {
		opts.OnConnect = cfg.MqttHandlers.OnConnect
	}
	if cfg.MqttHandlers.OnConnectionLost != nil {
		opts.OnConnectionLost = cfg.MqttHandlers.OnConnectionLost
	}
	if cfg.MqttHandlers.OnReconnecting != nil {
		opts.OnReconnecting = cfg.MqttHandlers.OnReconnecting
	}
	if cfg.MqttHandlers.OnConnectAttempt != nil {
		opts.OnConnectAttempt = cfg.MqttHandlers.OnConnectAttempt
	}
	return opts, nil
}

func (c *Client) CustomSettings() error {
	return nil
}

func (c *Client) Connect() (bool, error) {

	if token := c.Client.Connect(); token.Wait() && token.Error() != nil {
		return false, token.Error()
	} else {
		if t, ok := token.(connectToken); ok {
			return t.SessionPresent(), nil
		}
	}
	return false, nil
}

func (c *Client) Close() error {
	if c.Client.IsConnected() {
		c.Client.Disconnect(250)
	}
	return nil
}

func (c *Client) Write(topic string, value []byte) error {
	if !c.Client.IsConnected() {
		return paho.ErrNotConnected
	}
	token := c.Client.Publish(topic, c.QoS, c.Retain, value)
	if !token.WaitTimeout(c.Timeout) {
		return ErrPublishTimeout
	}
	return token.Error()
}

func (c *Client) WriteJson(topic string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Write(topic, data)
}

func (c *Client) Subscribe(topic string, qos byte, handler paho.MessageHandler) error {
	token := c.Client.Subscribe(topic, qos, handler)
	token.Wait()
	return token.Error()
}

func (c *Client) SubscribeMultiple(filters map[string]byte, handler paho.MessageHandler) error {
	token := c.Client.SubscribeMultiple(filters, handler)
	token.Wait()
	return token.Error()
}

func (c *Client) ChannelSubscribe(topic string, qos byte, ch chan paho.Message) error {
	handler := func(client paho.Client, msg paho.Message) {
		ch <- msg
	}
	token := c.Client.Subscribe(topic, qos, handler)
	token.Wait()
	return token.Error()
}

func (c *Client) AddRoute(topic string, handler paho.MessageHandler) {
	c.Client.AddRoute(topic, handler)
}
