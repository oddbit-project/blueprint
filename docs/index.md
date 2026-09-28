# Blueprint

Blueprint is a modular Go framework for building web applications and services. It gives you an
application container with graceful shutdown, configuration, structured logging, a Gin-based HTTP
server with security middleware and sessions, typed database repositories, and providers for
messaging, storage, authentication and observability.

## Install only what you use

The core module holds the container, configuration, logging, the database layer (`dbx`, `db`) and
utilities. Each provider is a separate Go module, so an application only pulls in the dependencies
of the providers it imports:

```bash
go get github.com/oddbit-project/blueprint                        # core
go get github.com/oddbit-project/blueprint/provider/httpserver     # HTTP server
go get github.com/oddbit-project/blueprint/provider/pgsql          # PostgreSQL driver
```

| Area | Modules (`github.com/oddbit-project/blueprint/provider/...`) |
|---|---|
| HTTP | `httpserver` |
| Databases | `pgsql`, `sqlite`, `clickhouse` |
| Messaging | `franz` (Kafka), `kafka` (legacy), `mqtt`, `nats` |
| Storage & cache | `redis`, `etcd`, `s3` |
| Authentication | `jwtprovider`, `hmacprovider`, `htpasswd` |
| Observability | `metrics`, `prometheus` |
| Email | `smtp` |

TLS, the key-value store interface and the rate limiter live in the core module
(`provider/tls`, `provider/kv`, `provider/ratelimiter`).

## A minimal application

An HTTP server run by the application container, which stops it cleanly on SIGINT/SIGTERM:

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/oddbit-project/blueprint"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/httpserver"
    "github.com/oddbit-project/blueprint/utils"
)

func main() {
    utils.PanicOnError(log.Configure(log.NewDefaultConfig()))
    logger := log.New("app")

    app := blueprint.NewContainer(nil)

    cfg := httpserver.NewServerConfig()
    cfg.Host = "localhost"
    cfg.Port = 8080
    server, err := cfg.NewServer(logger)
    app.AbortFatal(err)

    server.Route().GET("/hello", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "hello"})
    })

    // runs on shutdown, after the application context is cancelled
    blueprint.RegisterDestructor(func() error {
        return server.Shutdown(app.GetContext())
    })

    app.Run(func(any) error {
        go func() {
            app.AbortFatal(server.Start())
        }()
        return nil
    })
}
```

A fuller version reading its configuration from a JSON file is in
[`samples/application`](https://github.com/oddbit-project/blueprint/tree/main/samples/application);
the [`samples`](https://github.com/oddbit-project/blueprint/tree/main/samples) directory has
runnable programs for most providers.

## Where to go next

| Section | What's there |
|---|---|
| **Getting Started** | [Application container](container.md), [configuration](config/config.md), [logging](log/logging.md) |
| **Database** | [Overview](db/index.md): `dbx` repositories, the `gohan` query builder, migrations, drivers, and the [db → dbx migration guide](db/migrating-to-dbx.md) |
| **HTTP Server** | [Server, routing and middleware](provider/httpserver/index.md), authentication, security headers and CSRF, sessions, validation |
| **Security & Auth** | [Credentials](crypt/secure-credentials.md), [password hashing](crypt/password-hashing.md), [TLS](provider/tls.md), [JWT](provider/jwtprovider.md), [HMAC](provider/hmacprovider.md), [rate limiting](provider/ratelimiter.md) |
| **Integrations** | Messaging ([Franz/Kafka](provider/franz.md), [MQTT](provider/mqtt.md), [NATS](provider/nats.md)), storage ([Redis](provider/redis.md), [etcd](provider/etcd.md), [S3](provider/s3.md)), [metrics](provider/metrics.md), [SMTP](provider/smtp.md) |
| **Utilities** | [Periodic runner](runner/runner.md), [batch writer](batchwriter/batchwriter.md), [thread pool](threadpool/threadpool.md), helper packages |
