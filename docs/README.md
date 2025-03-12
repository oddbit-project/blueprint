# Blueprint Documentation

This directory contains comprehensive documentation for the Blueprint framework.

## Recent Documentation Updates

### New Documentation Added

- **Secure Credentials System** - Added documentation for the secure credential handling system in `crypt/secure-credentials.md`
- **TLS Provider** - Added documentation for TLS client and server configurations in `provider/tls.md`
- **HTTP Request Utilities** - Added documentation for HTTP utility functions in `provider/httpserver/request.md`

### Updated Documentation

- **Repository Pattern** - Updated `db/repository.md` with Count() and CountWhere() functionality
- **Kafka Client** - Enhanced `provider/kafka.md` with improved producer/consumer examples and security features
- **Main Index** - Reorganized the documentation index for better navigation

## Documentation Structure

The documentation is organized by module:

- `config/` - Configuration system documentation
- `crypt/` - Security and cryptography documentation
- `db/` - Database access and repository pattern documentation
- `log/` - Structured logging documentation
- `provider/` - Service provider documentation
  - `clickhouse/` - ClickHouse client documentation
  - `httpserver/` - HTTP server documentation
  - `kafka/` - Kafka client documentation
  - `mqtt/` - MQTT client documentation
  - `pgsql/` - PostgreSQL client documentation
  - `tls/` - TLS configuration documentation

## Contribution Guidelines

When updating documentation, please:

1. Keep examples up to date with the actual code
2. Include practical usage examples for each feature
3. Document security implications and best practices
4. Add any new modules or major features to the main index
5. Link related documentation when appropriate

## Future Documentation Plans

- Add more comprehensive examples for each module
- Add architecture diagrams for complex features
- Enhance security documentation with more best practices
- Add a troubleshooting section for common issues