# Blueprint Documentation

## Getting Started

Blueprint is a modular Go framework. Starting from v0.8.0, you can import only the components you need:

```bash
# Install core framework
go get github.com/oddbit-project/blueprint

# Install specific providers
go get github.com/oddbit-project/blueprint/provider/httpserver
go get github.com/oddbit-project/blueprint/provider/kafka
go get github.com/oddbit-project/blueprint/provider/pgsql
```

All existing imports continue to work without changes due to Go module rewrite rules.

## Development & Releases

- [Release Process](release-process.md) - How to create releases and manage independent provider versioning

## Configuration

- [Config](config/config.md)

## Database

- [Database Package Overview](db/index.md)
- [Structs and Tags](db/structs-and-tags.md)
- [Client Interface](db/client.md)
- [Repository Pattern](db/repository.md)
- [Data Grid System](db/dbgrid.md)
- [Field Specifications](db/fields.md)
- [Query Builder](db/query-builder.md)
- [Database Functions](db/functions.md)
- [Migration System](db/migrations.md)
- [SQL Update API](db/sql-update-api.md)

## Security

- [Password Hashing](crypt/password-hashing.md)
- [Secure Credentials](crypt/secure-credentials.md)
- [TLS](provider/tls.md)

## Providers

### Message Queues & Communication
- [Kafka](provider/kafka.md)
- [MQTT](provider/mqtt.md)
- [NATS](provider/nats.md)

### Databases & Storage
- [ClickHouse](provider/clickhouse.md)
- [etcd](provider/etcd.md)
- [PostgreSQL](provider/pgsql.md)
- [Redis](provider/redis.md)
- [S3 Storage](provider/s3.md)

### Web & HTTP
- Metrics *(documentation pending)*

### Authentication & Security
- [HMAC Provider](provider/hmacprovider.md)
- [htpasswd](provider/htpasswd.md)
- [JWT Provider](provider/jwtprovider.md)

### Utilities
- [SMTP](provider/smtp.md)

## Logging

- [Logging](log/logging.md)

## HTTP Server

- [HTTP Server Framework](provider/httpserver/index.md) - Complete overview and quick start
- [API Reference](provider/httpserver/api-reference.md) - Complete server API documentation
- [Middleware Components](provider/httpserver/middleware.md) - All middleware and utilities
- [Integration Examples](provider/httpserver/examples.md) - REST API, web app, and microservice examples
- [Troubleshooting Guide](provider/httpserver/troubleshooting.md) - Debugging and common issues
- [Performance Guide](provider/httpserver/performance.md) - Optimization and production deployment
- [Authentication](provider/httpserver/auth.md) - Token and JWT authentication providers
- [Security & Headers](provider/httpserver/security.md) - Security middleware and CSRF protection
- [Session Management](provider/httpserver/session.md) - Cookie-based session system
- [Request Utilities](provider/httpserver/request.md) - Request helper functions

## Utilities

- [BatchWriter](batchwriter/batchwriter.md)
- [ThreadPool](threadpool/threadpool.md)