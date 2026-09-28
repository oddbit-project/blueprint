# Prometheus Provider

The Prometheus provider exposes Prometheus metrics through the Blueprint [HTTP Server provider](httpserver/index.md)
(gin). It can run as a dedicated metrics server, or add a metrics endpoint to an HTTP server the application already
runs.

Each server (or each call to `Register`) uses its **own private `prometheus.Registry`**, not the Prometheus client's
global default registry. The registry always contains the Go runtime and process collectors, plus any collectors you
pass in. Metrics registered with `prometheus.MustRegister()` or `promauto` land in the global registry and are **not**
exposed by this provider.

> **prometheus or metrics?** The [Metrics provider](metrics.md) is a minimal `net/http` listener that exposes the global
> default registry. See [Comparison with the Metrics provider](#comparison-with-the-metrics-provider).

## Features

- **Standalone metrics server**: a dedicated `httpserver.Server` serving the metrics endpoint (`localhost:2220` by default)
- **Can be disabled from configuration**: with `Enabled = false`, `NewServer` returns no server
- **Attach to an existing server**: `Register()` adds the endpoint to any `*httpserver.Server`
- **Isolated registry**: a new `prometheus.Registry` per server, pre-loaded with Go and process collectors
- **Custom collectors**: pass any number of `prometheus.Collector` values, or add them later through the registry
- **HTTP server features**: TLS, timeouts, trusted proxies and structured request logging from the HTTP Server provider

## Installation

```bash
go get github.com/oddbit-project/blueprint/provider/prometheus
```

## Configuration

### Basic Configuration

```go
package main

import (
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/prometheus"
)

func main() {
	logger := log.New("metrics")

	cfg := prometheus.NewConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 9100
	cfg.Endpoint = "/metrics"

	server, err := cfg.NewServer(logger)
	if err != nil {
		logger.Fatal(err, "could not create prometheus server")
	}

	// blocks until the server is shut down or fails
	if err := server.Start(); err != nil {
		logger.Fatal(err, "prometheus server failed")
	}
}
```

### JSON Configuration

`Config` embeds `httpserver.ServerConfig`, which in turn embeds `tls.ServerConfig`, so all fields sit at the same
level:

```json
{
  "prometheus": {
    "enabled": true,
    "endpoint": "/metrics",
    "host": "localhost",
    "port": 2220,
    "readTimeout": 30,
    "writeTimeout": 60,
    "debug": false,
    "serverName": "prometheus",
    "trustedProxies": [],
    "tlsEnable": false,
    "tlsCert": "",
    "tlsKey": "",
    "tlsKeyPassword": "",
    "tlsKeyPasswordEnvVar": "",
    "tlsKeyPasswordFile": "",
    "tlsAllowedCACerts": [],
    "tlsCipherSuites": [],
    "tlsMinVersion": "",
    "tlsMaxVersion": "",
    "tlsAllowedDNSNames": []
  }
}
```

Loading it with the JSON config provider:

```go
package main

import (
	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/prometheus"
)

func main() {
	logger := log.New("metrics")

	cfg, err := provider.NewJsonProvider("config.json")
	if err != nil {
		logger.Fatal(err, "could not load configuration")
	}

	// start from the defaults so that omitted keys keep their default value
	promConfig := prometheus.NewConfig()
	if err := cfg.GetKey("prometheus", promConfig); err != nil {
		logger.Fatal(err, "invalid prometheus configuration")
	}

	server, err := promConfig.NewServer(logger)
	if err != nil {
		logger.Fatal(err, "could not create prometheus server")
	}
	if server == nil {
		// "enabled": false
		logger.Info("prometheus server disabled")
		return
	}
	if err := server.Start(); err != nil {
		logger.Fatal(err, "prometheus server failed")
	}
}
```

## Configuration Options

| Field                | JSON key         | Type       | Default        | Description                                                      |
|----------------------|------------------|------------|----------------|------------------------------------------------------------------|
| `Enabled`            | `enabled`        | `bool`     | `true`         | When `false`, `NewServer` returns a `nil` server (see Notes)      |
| `Endpoint`           | `endpoint`       | `string`   | `"/metrics"`   | Path of the metrics endpoint; empty falls back to `/metrics`     |
| `Host`               | `host`           | `string`   | `"localhost"`  | Listen address; empty listens on all interfaces                  |
| `Port`               | `port`           | `int`      | `2220`         | Listen port; zero falls back to `2220`                           |
| `ReadTimeout`        | `readTimeout`    | `int`      | `30`           | `http.Server.ReadTimeout`, in seconds                            |
| `WriteTimeout`       | `writeTimeout`   | `int`      | `60`           | `http.Server.WriteTimeout`, in seconds                           |
| `Debug`              | `debug`          | `bool`     | `false`        | gin debug mode; when `false`, gin is switched to release mode     |
| `ServerName`         | `serverName`     | `string`   | `"prometheus"` | Server name, used for the request logger when no logger is given |
| `TrustedProxies`     | `trustedProxies` | `[]string` | `[]`           | Trusted proxy addresses for gin                                  |
| `TLSEnable` and other `tls*` fields | see [TLS provider](tls.md) | | disabled | Server TLS options (embedded `tls.ServerConfig`)         |

`Validate()` replaces a zero `Port` with `DefaultPort` (`2220`) and then delegates to
`httpserver.ServerConfig.Validate()`, which replaces an empty `ServerName` with `"http"` and non-positive timeouts with
`30`/`60`, and rejects ports outside `0`–`65535`. `Host` is not defaulted by `Validate()`: an empty `Host` listens on
all interfaces, so start from `NewConfig()` to get the `localhost` binding.

## Usage Examples

### Standalone Metrics Server with Custom Collectors

Collectors passed to `NewServer` are registered on the server's private registry. More can be added later via
`Registry()`:

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/prometheus"
	prom "github.com/prometheus/client_golang/prometheus"
)

func main() {
	_ = log.Configure(log.NewDefaultConfig())
	logger := log.New("metrics")

	jobsProcessed := prom.NewCounterVec(
		prom.CounterOpts{
			Name: "app_jobs_processed_total",
			Help: "Number of processed jobs by result",
		},
		[]string{"result"},
	)

	server, err := prometheus.NewServer(prometheus.NewConfig(), logger, jobsProcessed)
	if err != nil {
		logger.Fatal(err, "could not create prometheus server")
	}

	// register an additional collector after creation
	queueSize := prom.NewGauge(prom.GaugeOpts{
		Name: "app_queue_size",
		Help: "Number of queued jobs",
	})
	server.Registry().MustRegister(queueSize)

	go func() {
		// Start returns nil after Shutdown() is called
		if err := server.Start(); err != nil {
			logger.Fatal(err, "prometheus server failed")
		}
	}()
	logger.Info("metrics available at http://localhost:2220/metrics")

	// application work
	jobsProcessed.WithLabelValues("ok").Inc()
	queueSize.Set(3)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error(err, "prometheus server shutdown")
	}
}
```

### Adding the Endpoint to an Existing HTTP Server

`Register` adds a `GET` route to an existing `*httpserver.Server` and returns the registry backing it:

```go
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/prometheus"
	prom "github.com/prometheus/client_golang/prometheus"
)

func main() {
	logger := log.New("api")

	httpConfig := httpserver.NewServerConfig()
	httpConfig.Host = "localhost"
	httpConfig.Port = 8089

	server, err := httpserver.NewServer(httpConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not create http server")
	}

	requests := prom.NewCounterVec(
		prom.CounterOpts{
			Name: "app_requests_total",
			Help: "Total number of requests by endpoint",
		},
		[]string{"endpoint"},
	)

	// metrics now available at http://localhost:8089/metrics
	registry := prometheus.Register(server, "/metrics", requests)

	// the returned registry accepts further collectors
	registry.MustRegister(prom.NewGauge(prom.GaugeOpts{
		Name: "app_build_info",
		Help: "Build information",
	}))

	server.Route().GET("/hello", func(c *gin.Context) {
		requests.WithLabelValues("/hello").Inc()
		c.JSON(http.StatusOK, gin.H{"message": "hello!"})
	})

	if err := server.Start(); err != nil {
		logger.Fatal(err, "http server failed")
	}
}
```

### Protecting the Endpoint

gin applies middleware only to routes registered **after** it is added, and `NewServer` registers the metrics route
before it returns. To put authentication in front of the endpoint, create the HTTP server yourself, add the middleware,
then call `Register`:

```go
package main

import (
	"os"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/prometheus"
)

func main() {
	logger := log.New("metrics")

	promConfig := prometheus.NewConfig()
	server, err := promConfig.ServerConfig.NewServer(logger)
	if err != nil {
		logger.Fatal(err, "could not create http server")
	}

	// middleware first...
	if err := server.ProcessOptions(
		httpserver.WithAuthToken("", os.Getenv("METRICS_TOKEN")), // X-API-Key header
	); err != nil {
		logger.Fatal(err, "could not configure http server")
	}

	// ...then the metrics route
	prometheus.Register(server, promConfig.Endpoint)

	if err := server.Start(); err != nil {
		logger.Fatal(err, "http server failed")
	}
}
```

## API Reference

### Constants

| Name              | Value        | Description                          |
|-------------------|--------------|--------------------------------------|
| `DefaultEndpoint` | `"/metrics"`  | Default endpoint path                                  |
| `DefaultHost`     | `"localhost"` | Default host set by `NewConfig()`                      |
| `DefaultPort`     | `2220`        | Default port set by `NewConfig()` and by `Validate()`  |

### Types

```go
type Config struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
	httpserver.ServerConfig
}

// Server wraps httpserver.Server for prometheus metrics
type Server struct {
	*httpserver.Server
	// unexported fields
}
```

Because `Server` embeds `*httpserver.Server`, its fields (`Config`, `Router`, `Server`, `Logger`) and methods
(`Route()`, `Group()`, `AddMiddleware()`, `UseAuth()`, ...) are available on the Prometheus server as well.

### Functions and Methods

```go
func NewConfig() *Config
```

Returns a `Config` built from `httpserver.NewServerConfig()`, with `Host` set to `"localhost"`, `Port` to `2220`,
`ServerName` to `"prometheus"`, `Enabled` to `true` and `Endpoint` to `/metrics`.

```go
func (c *Config) Validate() error
```

Defaults a zero `Port` to `2220` and validates the embedded `httpserver.ServerConfig` (see
[Configuration Options](#configuration-options)).

```go
func NewServer(cfg *Config, logger *log.Logger, cs ...prometheus.Collector) (*Server, error)
func (c *Config) NewServer(logger *log.Logger, cs ...prometheus.Collector) (*Server, error)
```

Create a dedicated metrics server. A nil `cfg` is replaced with `NewConfig()`; a nil `logger` makes the HTTP server
create its own request logger. If `cfg.Enabled` is `false`, they return `nil, nil`: no server is created, so check the
returned server for `nil` before calling `Start`. Otherwise the function validates the config, creates an `httpserver.Server`, creates a new
`prometheus.Registry` holding the process collector, the Go collector and the collectors in `cs`, and registers
`GET <Endpoint>` (falling back to `/metrics` when empty). Registration uses `MustRegister`, so an invalid or duplicate
collector panics.

```go
func Register(server *httpserver.Server, endpoint string, cs ...prometheus.Collector) *prometheus.Registry
```

Adds `GET <endpoint>` (or `/metrics` when `endpoint` is empty) to an existing HTTP server, backed by a new registry
holding the process collector, the Go collector and the collectors in `cs`. Returns that registry. `Register` takes no
`Config`, so `Enabled` does not apply to it; check the flag yourself before calling it. Panics if a collector
cannot be registered or if the route already exists on the server.

```go
func (s *Server) Registry() *prometheus.Registry
```

Returns the server's registry, for registering additional collectors.

```go
func (s *Server) Start() error
```

Starts the underlying HTTP server and blocks. Returns `nil` after `Shutdown()`, or the listener error otherwise.

```go
func (s *Server) Shutdown(ctx context.Context) error
```

Gracefully shuts the underlying HTTP server down.

## Comparison with the Metrics provider

|                               | `provider/prometheus`                                          | `provider/metrics`                                   |
|-------------------------------|----------------------------------------------------------------|------------------------------------------------------|
| HTTP stack                    | Blueprint `httpserver` (gin), with its request logging         | `net/http` `ServeMux`                                |
| Registry                      | A new private `prometheus.Registry` per server/`Register` call | `prometheus.DefaultGatherer` (or any `Gatherer` via `NewCustomServer`) |
| Go / process collectors       | Always registered on the private registry                      | Whatever the default registry holds                  |
| Registering your metrics      | Pass collectors to `NewServer`/`Register`, or use `Registry()` | `prometheus.MustRegister` / `promauto`               |
| Attach to an existing server  | Yes, `Register(httpServer, endpoint, ...)`                     | No                                                   |
| Default address               | `localhost:2220`                                               | `localhost:2201`                                     |
| Default timeouts (read/write) | 30s / 60s                                                      | 600s / 600s                                          |
| `HandlerOpts`                 | Fixed (`promhttp.HandlerOpts{}`)                               | Configurable via `NewCustomServer`                   |
| HTTP methods on the endpoint  | `GET` only                                                     | Any                                                  |
| Nil config                    | Defaults from `NewConfig()`                                    | Error (`ErrNilConfig`)                               |

Use this provider when the application already runs a Blueprint HTTP server, when you want the metrics on the same
port as the API, or when you want an isolated registry. Use the [Metrics provider](metrics.md) when metrics are
recorded in the global default registry (including by third-party libraries) or when you want a minimal listener.

## Notes

- **Global registry is not exposed**: metrics created with `promauto` or registered with `prometheus.MustRegister` go
  to `prometheus.DefaultRegisterer` and do not appear on this provider's endpoint. Register them on the provider's
  registry instead.
- **Disabled server is `nil`**: with `Enabled = false`, `NewServer` returns `nil, nil`; calling `Start` on the result
  without a `nil` check panics. A `Config` that was not created with `NewConfig()` (for example `&prometheus.Config{}`
  filled from JSON without an `"enabled"` key) has `Enabled = false` and is therefore disabled. `Register` ignores
  `Enabled`.
- **Binds `localhost` by default**: `NewConfig()` sets `Host` to `"localhost"`, so the endpoint is not reachable from
  other hosts. Set `Host` to `""` or `"0.0.0.0"` to listen on all interfaces (for example, so that a Prometheus
  server on another host can scrape it).
- **Every scrape is logged**: the HTTP server's request logging middleware logs each request to the endpoint.
- **gin release mode is global**: with `Debug = false` (the default), creating the server calls
  `gin.SetMode(gin.ReleaseMode)`, which affects every gin engine in the process.
- **One registry per call**: each `NewServer`/`Register` call creates a separate registry. Calling `Register` twice on
  the same server with the same endpoint panics (duplicate gin route).
- **Duplicate collectors panic**: registering the same collector twice in one registry, or passing another Go or process
  collector (they are already registered), panics.

## See Also

- [Metrics Provider](metrics.md) - Minimal `net/http` metrics listener using the global registry
- [HTTP Server](httpserver/index.md) - The underlying HTTP server provider
- [Authentication](httpserver/auth.md) - Token and JWT authentication middleware
- [TLS Configuration](tls.md) - TLS server options
- [Configuration Management](../config/config.md) - Loading configuration from JSON or environment
