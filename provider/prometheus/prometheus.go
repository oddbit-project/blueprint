package prometheus

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server wraps httpserver.Server for prometheus metrics
type Server struct {
	*httpserver.Server
	registry *prometheus.Registry
}

// NewServer creates a new prometheus server using httpserver
//
// Example usage:
//
//	cfg := prometheus.NewConfig()
//	server, err := prometheus.NewServer(cfg, logger)
//	if err != nil {
//	  log.Fatal(err)
//	}
//	server.Start()
func NewServer(cfg *Config, logger *log.Logger, cs ...prometheus.Collector) (*Server, error) {
	if cfg == nil {
		cfg = NewConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	httpServer, err := httpserver.NewServer(&cfg.ServerConfig, logger)
	if err != nil {
		return nil, err
	}

	// create a custom registry to avoid global state issues
	registry := prometheus.NewRegistry()

	// register default collectors
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	// register custom collectors
	for _, c := range cs {
		registry.MustRegister(c)
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	// register prometheus handler using gin.WrapH
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	httpServer.Router.GET(endpoint, gin.WrapH(handler))

	return &Server{
		Server:   httpServer,
		registry: registry,
	}, nil
}

// NewServer creates and returns a new Server instance with the given logger and optional prometheus collectors.
func (c *Config) NewServer(logger *log.Logger, cs ...prometheus.Collector) (*Server, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return NewServer(c, logger, cs...)
}

// Register adds the prometheus metrics endpoint to an existing httpserver
//
// Example usage:
//
//	httpServer, _ := httpserver.NewServer(cfg, logger)
//	registry := prometheus.Register(httpServer, "/metrics")
//	registry.MustRegister(myCollector)
func Register(server *httpserver.Server, endpoint string, cs ...prometheus.Collector) *prometheus.Registry {
	registry := prometheus.NewRegistry()

	// register default collectors
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	// register custom collectors
	for _, c := range cs {
		registry.MustRegister(c)
	}

	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	server.Router.GET(endpoint, gin.WrapH(handler))

	return registry
}

// Registry returns the prometheus registry for registering additional collectors
func (s *Server) Registry() *prometheus.Registry {
	return s.registry
}

// Start starts the prometheus server (blocking)
func (s *Server) Start() error {
	return s.Server.Start()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}
