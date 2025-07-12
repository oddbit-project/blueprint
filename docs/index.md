# Blueprint documentation

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

- [Secure Credentials](crypt/secure-credentials.md)
- [htpasswd Authentication](provider/htpasswd.md)
- [TLS](provider/tls.md)

## Providers

- [Clickhouse](provider/clickhouse.md)
- [htpasswd](provider/htpasswd.md)
- [Kafka](provider/kafka.md)
- [PostgreSQL](provider/pgsql.md)
- [MQTT](provider/mqtt.md)
- [NATS](provider/nats.md)
- [TLS](provider/tls.md)
- [JWT Provider](provider/jwtprovider.md)

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