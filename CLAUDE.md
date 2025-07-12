# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test Commands

- Run all tests: `go test -v ./...`
- Run tests in specific package: `go test -v ./path/to/package/...`
- Run with race detection: `go test -v -race ./...`
- Run integration tests locally: `go test -v -tags=integration ./...`
- Docker-based tests: `make test`

## Code Style Guidelines

- **Imports**: Standard library first, third-party next, local packages last
- **Error handling**: Early returns, constants prefixed with `Err`, use utils.Error
- **Naming**: PascalCase (exported), camelCase (unexported), short receiver names
- **Types**: Prefer interfaces for decoupling, use repository pattern
- **Documentation**: Comment exported functions, document complex logic
- **Testing**: Use testify for assertions, table-driven tests
- **Context**: Pass as first parameter in functions
- **Package structure**: Domain-driven design approach