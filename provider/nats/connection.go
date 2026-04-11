package nats

import (
	"time"

	"github.com/nats-io/nats.go"
	"github.com/oddbit-project/blueprint/crypt/secure"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
)

// connectParams is the internal, shared set of inputs needed to open a NATS
// connection. Both the core-NATS consumer/producer and the JetStream clients
// build one of these and call connect().
type connectParams struct {
	URL      string
	Name     string
	AuthType string
	Username string
	Cred     secure.DefaultCredentialConfig
	TLS      tlsProvider.ClientConfig

	PingInterval uint // seconds
	MaxPingsOut  uint
	Timeout      uint // milliseconds
}

// connect opens a NATS connection using the shared parameters. It transparently
// handles secure credential loading for AuthTypeBasic/Token and TLS wiring.
// The caller is responsible for logging; connect only returns errors.
//
// Credential hygiene note: the secure.Credential buffer is Clear'd before
// return, but by that point the plaintext has been copied into Go strings
// (opts.Password / opts.Token) and retained internally by nats.Conn. Go
// strings are immutable and cannot be zeroed, so the Clear() call is a
// best-effort reduction of the plaintext window rather than a guarantee.
// Fully eliminating the plaintext exposure would require using a callback-
// based auth mechanism on nats.Options which is not currently wired up.
func connect(p connectParams) (*nats.Conn, error) {
	var (
		credential *secure.Credential
		password   string
		err        error
	)

	switch p.AuthType {
	case AuthTypeBasic, AuthTypeToken:
		key, kerr := secure.GenerateKey()
		if kerr != nil {
			return nil, kerr
		}
		if credential, err = secure.CredentialFromConfig(p.Cred, key, true); err != nil {
			return nil, err
		}
		// Ensure the encrypted credential buffer is Clear'd on every return
		// path (including TLS errors and panics), not just the normal one.
		defer credential.Clear()
		if password, err = credential.Get(); err != nil {
			return nil, err
		}
	}

	opts := nats.Options{
		Url:            p.URL,
		AllowReconnect: true,
		MaxReconnect:   DefaultConnectRetry,
		ReconnectWait:  DefaultTimeout,
		Name:           p.Name,
	}

	switch p.AuthType {
	case AuthTypeBasic:
		opts.User = p.Username
		opts.Password = password
	case AuthTypeToken:
		opts.Token = password
	}

	if tls, terr := p.TLS.TLSConfig(); terr != nil {
		return nil, terr
	} else if tls != nil {
		opts.TLSConfig = tls
	}

	if p.PingInterval > 0 {
		opts.PingInterval = time.Duration(p.PingInterval) * time.Second
	}
	if p.MaxPingsOut > 0 {
		opts.MaxPingsOut = int(p.MaxPingsOut)
	}
	if p.Timeout > 0 {
		opts.Timeout = time.Duration(p.Timeout) * time.Millisecond
	}

	conn, err := opts.Connect()
	if err != nil {
		return nil, err
	}
	return conn, nil
}
