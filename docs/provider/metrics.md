# Metrics Provider

The metrics provider runs a small, standalone HTTP server that exposes Prometheus metrics. It is a thin wrapper around
`net/http` and the Prometheus client library (`promhttp`): it serves a single endpoint (`/metrics` by default) on a
dedicated port, with optional TLS, and nothing else.

By default the endpoint exposes the Prometheus client's **global default registry** (`prometheus.DefaultGatherer`), so
any metric registered with `prometheus.MustRegister()` or created with `promauto` is published automatically, together
with the Go runtime and process metrics that the Prometheus client library registers there on its own.

> **metrics or prometheus?** Blueprint has two providers that serve Prometheus metrics. This one uses plain `net/http`
> and the global default registry. The [Prometheus provider](prometheus.md) is built on the
> [HTTP Server provider](httpserver/index.md) (gin), uses its own private registry, and can also attach the endpoint
> to an existing HTTP server. See [Comparison with the Prometheus provider](#comparison-with-the-prometheus-provider).

## Features

- **Dedicated metrics listener**: serves the metrics endpoint on its own host and port
- **Global registry by default**: exposes everything registered in `prometheus.DefaultRegisterer`
- **Custom gatherer**: `NewCustomServer` accepts any `prometheus.Gatherer` and `promhttp.HandlerOpts`
- **TLS support**: embeds the [TLS provider](tls.md) server configuration, including client certificate verification
- **Graceful shutdown**: `Shutdown(ctx)` delegates to `http.Server.Shutdown`

## Installation

```bash
go get github.com/oddbit-project/blueprint/provider/metrics
```

## Configuration

### Basic Configuration

```go
package main

import (
	"log"

	"github.com/oddbit-project/blueprint/provider/metrics"
)

func main() {
	cfg := metrics.NewConfig()
	cfg.Host = "0.0.0.0"
	cfg.Port = 9100
	cfg.Endpoint = "/metrics"

	server, err := cfg.NewServer()
	if err != nil {
		log.Fatal(err)
	}

	// blocks until the server is shut down or fails
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
```

### JSON Configuration

`Config` embeds `tls.ServerConfig`, so the TLS fields sit at the same level as the other options:

```json
{
  "metrics": {
    "host": "localhost",
    "port": 2201,
    "endpoint": "/metrics",
    "readTimeout": 600,
    "writeTimeout": 600,
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
	"log"

	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/provider/metrics"
)

func main() {
	cfg, err := provider.NewJsonProvider("config.json")
	if err != nil {
		log.Fatal(err)
	}

	// start from the defaults so that omitted keys keep their default value
	metricsConfig := metrics.NewConfig()
	if err := cfg.GetKey("metrics", metricsConfig); err != nil {
		log.Fatal(err)
	}

	server, err := metricsConfig.NewServer()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(server.Start())
}
```

### TLS Configuration

```go
cfg := metrics.NewConfig()
cfg.TLSEnable = true
cfg.TLSCert = "/path/to/server.crt"
cfg.TLSKey = "/path/to/server.key"
cfg.TLSMinVersion = "TLS13"

// optional: require client certificates signed by these CAs (mutual TLS)
cfg.TLSAllowedCACerts = []string{"/path/to/ca.crt"}
```

The TLS settings are turned into a `*tls.Config` by `tls.ServerConfig.TLSConfig()` when the server is created; see the
[TLS provider](tls.md) for the meaning of each field. Accepted version strings are `"TLS12"` and `"TLS13"`.

## Configuration Options

| Field                | JSON key               | Type       | Default       | Description                                           |
|----------------------|------------------------|------------|---------------|-------------------------------------------------------|
| `Host`               | `host`                 | `string`   | `"localhost"` | Listen address                                        |
| `Port`               | `port`                 | `int`      | `2201`        | Listen port                                           |
| `Endpoint`           | `endpoint`             | `string`   | `"/metrics"`  | Path of the metrics endpoint (`http.ServeMux` pattern) |
| `ReadTimeout`        | `readTimeout`          | `int`      | `600`         | `http.Server.ReadTimeout`, in seconds                 |
| `WriteTimeout`       | `writeTimeout`         | `int`      | `600`         | `http.Server.WriteTimeout`, in seconds                |
| `TLSEnable`          | `tlsEnable`            | `bool`     | `false`       | Serve over TLS (embedded `tls.ServerConfig`)          |
| `TLSCert`            | `tlsCert`              | `string`   | `""`          | Server certificate file                               |
| `TLSKey`             | `tlsKey`               | `string`   | `""`          | Server private key file                               |
| `Password`           | `tlsKeyPassword`       | `string`   | `""`          | Private key password                                  |
| `PasswordEnvVar`     | `tlsKeyPasswordEnvVar` | `string`   | `""`          | Environment variable holding the key password         |
| `PasswordFile`       | `tlsKeyPasswordFile`   | `string`   | `""`          | File holding the key password                         |
| `TLSAllowedCACerts`  | `tlsAllowedCACerts`    | `[]string` | `nil`         | CA files; when set, client certificates are required  |
| `TLSCipherSuites`    | `tlsCipherSuites`      | `[]string` | `nil`         | Allowed cipher suites (provider defaults when empty)  |
| `TLSMinVersion`      | `tlsMinVersion`        | `string`   | `""`          | Minimum TLS version (`TLS13` when empty)              |
| `TLSMaxVersion`      | `tlsMaxVersion`        | `string`   | `""`          | Maximum TLS version                                   |
| `TLSAllowedDNSNames` | `tlsAllowedDNSNames`   | `[]string` | `nil`         | Allowed DNS names in client certificates              |

The defaults are those set by `NewConfig()`. `Validate()` (called by the server constructors) fills in some of them
for a partially populated `Config`: a zero `Port` becomes `2201`, and a `ReadTimeout`/`WriteTimeout` `<= 0` becomes
`600`. It does not fill in `Endpoint` (an empty value is rejected with `ErrMissingEndpoint`) or `Host` (an empty value
binds all interfaces), so start from `NewConfig()` to get the `localhost` binding.

## Usage Examples

### Exposing Application Metrics

Metrics registered in the default registry are served without any further wiring:

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oddbit-project/blueprint/provider/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// promauto registers the counter in prometheus.DefaultRegisterer
var jobsProcessed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "app_jobs_processed_total",
		Help: "Number of processed jobs by result",
	},
	[]string{"result"},
)

func main() {
	server, err := metrics.NewServer(metrics.NewConfig())
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		// Start returns nil after Shutdown() is called
		if err := server.Start(); err != nil {
			log.Fatal(err)
		}
	}()
	log.Println("metrics available at http://localhost:2201/metrics")

	// application work
	jobsProcessed.WithLabelValues("ok").Inc()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Println("metrics server shutdown:", err)
	}
}
```

### Using a Custom Registry

`NewCustomServer` serves any `prometheus.Gatherer` with the given `promhttp.HandlerOpts`. Use it to avoid the global
registry, or to tune the handler:

```go
package main

import (
	"log"

	"github.com/oddbit-project/blueprint/provider/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	requests := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_requests_total",
		Help: "Total number of requests",
	})
	registry.MustRegister(requests)

	cfg := metrics.NewConfig()
	server, err := metrics.NewCustomServer(cfg, registry, promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		MaxRequestsInFlight: 5,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(server.Start())
}
```

A `*prometheus.Registry` implements both `prometheus.Registerer` and `prometheus.Gatherer`. With a custom registry only
the collectors you register on it are exposed; the Go and process collectors are not added for you.

## API Reference

### Constants

| Name                  | Value                   | Description                          |
|-----------------------|-------------------------|--------------------------------------|
| `DefaultReadTimeout`  | `600`                   | Default read timeout (seconds)       |
| `DefaultWriteTimeout` | `600`                   | Default write timeout (seconds)      |
| `DefaultHost`         | `"localhost"`           | Default listen host                  |
| `DefaultPort`         | `2201`                  | Default listen port                  |
| `DefaultEndpoint`     | `"/metrics"`            | Default endpoint path                |
| `ErrNilConfig`        | `utils.Error("Config is nil")` | Returned when a nil `*Config` is passed |
| `ErrMissingEndpoint`  | `utils.Error("Endpoint is empty")` | Returned by `Validate` when `Endpoint` is empty |
| `ErrMissingTLSCert`   | `utils.Error("TLS is enabled but TLSCert or TLSKey is empty")` | Returned by `Validate` when `TLSEnable` is set without `TLSCert` and `TLSKey` |

### Types

```go
type Config struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Endpoint     string `json:"endpoint"`
	ReadTimeout  int    `json:"readTimeout"`
	WriteTimeout int    `json:"writeTimeout"`
	tlsProvider.ServerConfig
}

type Server struct {
	// unexported fields
}
```

### Functions and Methods

```go
func NewConfig() *Config
```

Returns a `Config` populated with the default values listed above, with TLS disabled.

```go
func (c *Config) Validate() error
```

Returns `ErrMissingEndpoint` if `Endpoint` is empty, and `ErrMissingTLSCert` if `TLSEnable` is true but `TLSCert` or
`TLSKey` is empty. Otherwise it applies defaults in place (`Port` `0` → `DefaultPort`, `ReadTimeout`/`WriteTimeout`
`<= 0` → `600`) and returns `nil`. `Host` is not defaulted. The TLS fields are checked later, by `TLSConfig()`, when
the server is created.

```go
func (c *Config) NewServer() (*Server, error)
func NewServer(cfg *Config) (*Server, error)
```

Create a server that exposes `prometheus.DefaultGatherer` with the default `promhttp.HandlerOpts{}`. Both are shorthands
for `NewCustomServer(cfg, prometheus.DefaultGatherer, promhttp.HandlerOpts{})`. `NewServer(nil)` returns
`ErrNilConfig`.

```go
func NewCustomServer(cfg *Config, gatherer prometheus.Gatherer, opts promhttp.HandlerOpts) (*Server, error)
```

Validates `cfg` with `Validate()`, builds the TLS configuration from the embedded `tls.ServerConfig` (returning
either error, if any), registers
`promhttp.HandlerFor(gatherer, opts)` on a new `http.ServeMux` under `cfg.Endpoint`, and prepares an `http.Server`
listening on `Host:Port` with the configured timeouts. The listener is not opened until `Start()` is called. Returns
`ErrNilConfig` if `cfg` is nil.

```go
func (s *Server) Start() error
```

Starts listening and blocks until the server stops. Uses `ListenAndServeTLS` when TLS is enabled, `ListenAndServe`
otherwise. Returns `nil` when the server was stopped with `Shutdown()`, or the listener error otherwise.

```go
func (s *Server) Shutdown(ctx context.Context) error
```

Gracefully shuts the server down via `http.Server.Shutdown(ctx)`.

## Comparison with the Prometheus provider

|                               | `provider/metrics`                          | `provider/prometheus`                                          |
|-------------------------------|---------------------------------------------|----------------------------------------------------------------|
| HTTP stack                    | `net/http` `ServeMux`                       | Blueprint `httpserver` (gin), with its request logging         |
| Registry                      | `prometheus.DefaultGatherer` (or any `Gatherer` via `NewCustomServer`) | A new private `prometheus.Registry` per server |
| Go / process collectors       | Whatever the default registry holds (the client library registers them) | Always registered on the private registry |
| Registering your metrics      | `prometheus.MustRegister` / `promauto`      | Pass collectors to `NewServer`/`Register`, or use `Registry()` |
| Attach to an existing server  | No                                          | Yes, `prometheus.Register(httpServer, endpoint, ...)`         |
| Default address               | `localhost:2201`                            | `localhost:2220`                                               |
| Default timeouts (read/write) | 600s / 600s                                 | 30s / 60s (httpserver defaults)                                |
| `HandlerOpts`                 | Configurable via `NewCustomServer`          | Fixed (`promhttp.HandlerOpts{}`)                               |
| HTTP methods on the endpoint  | Any                                         | `GET` only                                                     |
| Nil config                    | Error (`ErrNilConfig`)                      | Defaults from `NewConfig()`                                    |

Use this provider when metrics are recorded in the global default registry (including by third-party libraries that
register there), or when you want a minimal listener without the gin stack. Use the [Prometheus provider](prometheus.md)
when the application already runs a Blueprint HTTP server, or when you want an isolated registry.

## Notes

- **Endpoint must be a valid pattern**: `Endpoint` is passed to `http.ServeMux.Handle`. An empty value is rejected by
  `Validate()` with `ErrMissingEndpoint`; any other invalid pattern makes `NewCustomServer` panic. Standard `ServeMux`
  pattern rules apply, so a value ending in `/` matches the whole subtree.
- **TLS needs a certificate**: with `TLSEnable = true`, both `TLSCert` and `TLSKey` must be set, otherwise server
  creation fails with `ErrMissingTLSCert`.
- **Empty host**: an empty `Host` is not replaced by `DefaultHost`; the server listens on all interfaces.
- **Start blocks**: run it in a goroutine if the application has other work to do.
- **Default registry contents**: the Prometheus client library registers the Go and process collectors in the default
  registry at init time, so they appear on the endpoint without any configuration. Registering them again with
  `prometheus.MustRegister` panics with a duplicate registration error.
- **No authentication**: the endpoint is unauthenticated. Keep the default `localhost` binding, restrict access at the
  network level, or use TLS with `TLSAllowedCACerts` to require client certificates.

## See Also

- [Prometheus Provider](prometheus.md) - Metrics endpoint built on the HTTP Server provider
- [TLS Configuration](tls.md) - TLS server options
- [Configuration Management](../config/config.md) - Loading configuration from JSON or environment
