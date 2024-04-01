package metrics

import (
	"context"
	"errors"
	"fmt"
	tlsProvider "github.com/oddbit-project/blueprint/provider/tls"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

const (
	DefaultReadTimeout  = 600
	DefaultWriteTimeout = 600
	DefaultHost         = "localhost"
	DefaultPort         = 2201
	DefaultEndpoint     = "/metrics"
)

type Config struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Endpoint     string `json:"endpoint"`
	ReadTimeout  int    `json:"readTimeout"`
	WriteTimeout int    `json:"writeTimeout"`
	tlsProvider.ServerConfig
}

type Server struct {
	server *http.Server
}

func NewConfig() *Config {
	return &Config{
		Host:         DefaultHost,
		Port:         DefaultPort,
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
		Endpoint:     DefaultEndpoint,
		ServerConfig: tlsProvider.ServerConfig{
			TLSCert:            "",
			TLSKey:             "",
			TLSKeyPwd:          "",
			TLSAllowedCACerts:  nil,
			TLSCipherSuites:    nil,
			TLSMinVersion:      "",
			TLSMaxVersion:      "",
			TLSAllowedDNSNames: nil,
			TLSEnable:          false,
		},
	}
}

func (c *Config) Validate() error {
	return nil
}

func (c *Config) NewServer() (*Server, error) {
	return NewCustomServer(c, prometheus.DefaultGatherer, promhttp.HandlerOpts{})
}

func NewServer(cfg *Config) (*Server, error) {
	return cfg.NewServer()
}

// NewCustomServer creates a new custom server based on the provided config, Prometheus gatherer, and handler options.
// It validates the config using the Validate method, then creates a new http.ServeMux and registers the Prometheus handler with it.
// The server is created with the specified host, port, router, read timeout, write timeout, and TLS config.
// Finally, it returns a pointer to the created Server instance.
// Example usage:
//
//	cfg := &Config{...}
//	gatherer := prometheus.DefaultGatherer
//	opts := promhttp.HandlerOpts{...}
//	server, err := NewCustomServer(cfg, gatherer, opts)
//	if err != nil {
//	    // handle error
//	}
//	defer server.Shutdown(context.Background())
//	err = server.Start()
//	if err != nil {
//	    // handle error
//	}
func NewCustomServer(cfg *Config, gatherer prometheus.Gatherer, opts promhttp.HandlerOpts) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	tlsConfig, err := cfg.TLSConfig()
	if err != nil {
		return nil, err
	}
	router := http.NewServeMux()
	router.Handle(cfg.Endpoint, promhttp.HandlerFor(gatherer, opts))
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		TLSConfig:    tlsConfig,
	}

	return &Server{server: server}, nil
}

// Start starts the server and listens for incoming connections.
// It uses the http.ListenAndServe function if the server's TLSConfig is nil,
// otherwise it uses the http.ListenAndServeTLS function.
// The method returns an error if the server fails to start.
// If the server is shut down using the Shutdown method, the returned error is http.ErrServerClosed.
func (s *Server) Start() error {
	var err error
	if s.server.TLSConfig == nil {
		err = s.server.ListenAndServe()
	} else {
		err = s.server.ListenAndServeTLS("", "")
	}
	// when Shutdown() is called, the return error is http.ErrServerClosed
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server by calling the Shutdown method of the underlying http.Server
// It takes a context.Context object as a parameter, which can be used to control the shutdown process.
// The method returns an error if the shutdown process fails.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
