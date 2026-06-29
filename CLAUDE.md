# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test Commands

The core module and each `provider/*` are separate Go modules wired via `go.work`. `go test ./...` from
the repo root only covers the core module, so provider modules are tested in their own directory (the
Makefile loops over them in `test-all`/`test-providers`).

Most integration tests use testcontainers and gate on `testing.Short()` — they run by default and are
skipped with `-short`, so a Docker daemon is required for the default run. A few suites (`db`,
`provider/etcd`) additionally require the `integration` build tag.

- Unit tests only (no Docker): `go test -short -race ./...` (or `make test-unit`)
- All tests incl. testcontainers integration (needs Docker): `go test -race ./...` (or `make test` / `make test-integration`)
- Tests in a specific package: `go test -race ./path/to/package/...`
- A provider module: `cd provider/<name> && go test -race ./...`
- Build-tag-gated integration tests: `go test -tags=integration ./db/... ./provider/etcd/...`
- Per-provider container suites: `make test-clickhouse`, `make test-kafka`, `make test-nats`, `make test-mqtt`, `make test-pgsql`, `make test-s3`

## Code Style Guidelines

- **Imports**: Standard library first, third-party next, local packages last
- **Error handling**: Early returns, constants prefixed with `Err`, use utils.Error
- **Naming**: PascalCase (exported), camelCase (unexported), short receiver names
- **Types**: Prefer interfaces for decoupling, use repository pattern
- **Documentation**: Comment exported functions, document complex logic
- **Testing**: Use testify for assertions, table-driven tests
- **Context**: Pass as first parameter in functions
- **Package structure**: Domain-driven design approach