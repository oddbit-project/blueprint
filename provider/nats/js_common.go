package nats

import (
	"context"
	"slices"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/oddbit-project/blueprint/crypt/secure"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrMissingJSURL         = utils.Error("Missing JetStream URL")
	ErrMissingStreamName    = utils.Error("Missing JetStream stream name")
	ErrJSNoConsumer         = utils.Error("JetStream consumer not initialized")
	ErrAlreadyConsuming     = utils.Error("JetStream consumer is already consuming")
	ErrInvalidAckPolicy     = utils.Error("Invalid JetStream ack policy")
	ErrInvalidDeliverPolicy = utils.Error("Invalid JetStream deliver policy")
	ErrInvalidRetention     = utils.Error("Invalid JetStream retention policy")
	ErrInvalidStorage       = utils.Error("Invalid JetStream storage type")

	// DefaultJSSetupTimeout bounds stream/consumer lookup and create-or-update
	// operations performed during client construction. Publish operations use
	// the caller-supplied context.
	DefaultJSSetupTimeout = 10 * time.Second
)

// JSConnectionConfig holds the fields needed to open a NATS connection for
// JetStream. It mirrors the shape of ConsumerConfig/ProducerConfig so callers
// can configure auth/TLS identically.
type JSConnectionConfig struct {
	URL      string `json:"url"`
	AuthType string `json:"authType"`
	Username string `json:"username"`
	secure.DefaultCredentialConfig
	ClientName string `json:"clientName"`
	tlsProvider.ClientConfig

	PingInterval uint `json:"pingInterval"` // seconds
	MaxPingsOut  uint `json:"maxPingsOut"`
	Timeout      uint `json:"timeout"` // milliseconds
}

// Validate verifies the connection config.
func (c JSConnectionConfig) Validate() error {
	if len(c.URL) == 0 {
		return ErrMissingJSURL
	}
	if !slices.Contains(validAuthTypes, c.AuthType) {
		return ErrInvalidAuthType
	}
	return nil
}

// dial opens a NATS connection using the shared connect() helper. It validates
// the config and resolves a default client name when one is not provided.
func (cfg *JSConnectionConfig) dial(defaultName string) (*nats.Conn, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	name := cfg.ClientName
	if name == "" {
		name = defaultName
	}
	return connect(connectParams{
		URL:          cfg.URL,
		Name:         name,
		AuthType:     cfg.AuthType,
		Username:     cfg.Username,
		Cred:         cfg.DefaultCredentialConfig,
		TLS:          cfg.ClientConfig,
		PingInterval: cfg.PingInterval,
		MaxPingsOut:  cfg.MaxPingsOut,
		Timeout:      cfg.Timeout,
	})
}

// StreamConfig mirrors the subset of jetstream.StreamConfig that is typically
// configured from external sources. The full native type is accepted via
// Native when callers need advanced fields.
type StreamConfig struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Subjects    []string      `json:"subjects"`
	Retention   string        `json:"retention"` // "limits" | "interest" | "workqueue"
	Storage     string        `json:"storage"`   // "file" | "memory"
	MaxAge      time.Duration `json:"maxAge"`
	MaxMsgs     int64         `json:"maxMsgs"`
	MaxBytes    int64         `json:"maxBytes"`
	Replicas    int           `json:"replicas"`
	Duplicates  time.Duration `json:"duplicates"`

	// Native allows overriding with a fully-formed jetstream.StreamConfig.
	// When set, all other fields are ignored.
	Native *jetstream.StreamConfig `json:"-"`
}

func (s StreamConfig) toNative() (jetstream.StreamConfig, error) {
	if s.Native != nil {
		return *s.Native, nil
	}
	if s.Name == "" {
		return jetstream.StreamConfig{}, ErrMissingStreamName
	}

	out := jetstream.StreamConfig{
		Name:        s.Name,
		Description: s.Description,
		Subjects:    s.Subjects,
		MaxAge:      s.MaxAge,
		MaxMsgs:     s.MaxMsgs,
		MaxBytes:    s.MaxBytes,
		Replicas:    s.Replicas,
		Duplicates:  s.Duplicates,
	}

	switch s.Retention {
	case "", "limits":
		out.Retention = jetstream.LimitsPolicy
	case "interest":
		out.Retention = jetstream.InterestPolicy
	case "workqueue":
		out.Retention = jetstream.WorkQueuePolicy
	default:
		return jetstream.StreamConfig{}, ErrInvalidRetention
	}

	switch s.Storage {
	case "", "file":
		out.Storage = jetstream.FileStorage
	case "memory":
		out.Storage = jetstream.MemoryStorage
	default:
		return jetstream.StreamConfig{}, ErrInvalidStorage
	}

	return out, nil
}

// EnsureStream creates the stream if it does not exist, or updates it to match
// cfg. Use AutoCreateStream=false on producers to opt out and call this
// explicitly from setup code.
func EnsureStream(ctx context.Context, js jetstream.JetStream, cfg StreamConfig) (jetstream.Stream, error) {
	native, err := cfg.toNative()
	if err != nil {
		return nil, err
	}
	return js.CreateOrUpdateStream(ctx, native)
}
