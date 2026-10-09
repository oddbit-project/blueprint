# Blueprint Release Process

This document explains how to create releases for the Blueprint framework, including both coordinated releases and
independent provider versioning.

## Overview

Blueprint uses a modular architecture with independent versioning for each provider module. This allows:

- **Selective updates**: Users install only the providers they need
- **Independent releases**: Providers can be updated without changing core
- **Faster iteration**: Bug fixes and features ship independently
- **Reduced dependencies**: Smaller footprint for applications

## Repository Structure

```
blueprint/
├── go.mod                    # Core module (github.com/oddbit-project/blueprint)
├── provider/
│   ├── redis/go.mod         # github.com/oddbit-project/blueprint/provider/redis
│   ├── s3/go.mod            # github.com/oddbit-project/blueprint/provider/s3
│   ├── httpserver/go.mod    # github.com/oddbit-project/blueprint/provider/httpserver
│   └── ...                  # 13 total provider modules
└── go.work                  # Workspace for development
```

## Release Types

### 1. Provider-Only Release

When updating a single provider without core changes:

#### Steps:

1. **Make changes** to the specific provider
2. **Test the provider**:
   ```bash
   cd provider/redis
   go test -v ./...
   ```
3. **Tag only that provider**:
   ```bash
   git tag provider/redis/v0.8.1
   git push origin provider/redis/v0.8.1
   ```
4. **Create GitHub release** for the provider tag
5. **Update documentation** if needed

#### Example Scenarios:

- Redis provider bug fix → `provider/redis/v0.8.1`
- S3 provider new feature → `provider/s3/v0.8.2`
- SMTP provider security update → `provider/smtp/v0.8.3`

### 2. Core-Only Release

When updating core library without provider changes:

#### Steps:

1. **Make changes** to core modules (`/db`, `/config`, `/crypt`, etc.)
2. **Test core and all providers**:
   ```bash
   make test-all
   ```
3. **Tag core only**:
   ```bash
   git tag v0.8.1
   git push origin v0.8.1
   ```
4. **Create GitHub release** for the core tag

### 3. Coordinated Release

When updating both core and providers (major releases):

#### Steps:

1. **Make changes** across core and providers
2. **Run comprehensive tests**:
   ```bash
   make test-all
   make build-all
   ```
3. **Update dependencies** if needed:
   ```bash
   make update-deps VERSION=v0.9.0
   ```
4. **Tag all modules**:
   ```bash
   make tag-version VERSION=v0.9.0
   ```
5. **Push all tags**:
   ```bash
   git push origin v0.9.0
   git push origin --tags
   ```
6. **Create GitHub release** with comprehensive release notes

## Release Commands Reference

### Using Makefile

```bash
# Build and test everything
make build-all
make test-all

# Tag all modules with same version (coordinated release)
make tag-version VERSION=v0.9.0

# Update all provider dependencies to specific core version
make update-deps VERSION=v0.8.0

# Generate Software Bill of Materials
make sbom
```

### Manual Git Commands

```bash
# Tag core module
git tag v0.8.0

# Tag specific provider
git tag provider/redis/v0.8.1

# Push specific tag
git push origin provider/redis/v0.8.1

# Push all tags
git push origin --tags

# List existing tags
git tag --sort=-version:refname
```

## gohan/dbx release order

`dbx` and `provider/clickhouse`'s non-test code depend on the standalone module
`github.com/oddbit-project/gohan` (`dbx` directly; `provider/clickhouse` via `Client.Querier()`).
`gohan` is released and tagged independently of Blueprint, in its own repository. `make
tag-version VERSION=…` tags core and every provider module **at the same commit**, but each
provider's `go.mod` still `require`s an older core version (shadowed in this workspace by a local
`replace`, which a real consumer does not get) until that `require` is bumped. Run as a single
step, `tag-version` therefore publishes `provider/clickhouse` at a version whose declared core
dependency does not contain the current `dbx`/`gohan` code — a broken release for any consumer
who resolves the provider's `go.mod` normally (i.e. without the workspace's `replace`).

For a release that bumps `gohan`, adds to or changes `dbx`, or anything a provider's non-test
code imports from core, tag in this order instead of a single `tag-version` call:

1. **Tag `gohan` first**, in its own repository, if this release depends on a new `gohan`
   version.
2. **Tag core only**: `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. **Bump the core requirement** in the affected providers' `go.mod` — currently
   `provider/clickhouse`, `provider/pgsql` and `provider/sqlite` (`dbx`/`gohan`), and
   `provider/etcd`, `provider/franz`, `provider/kafka`, `provider/mqtt`, `provider/nats`,
   `provider/redis`, `provider/s3` and `provider/smtp` (`tls.ClientConfig.ValidateEnabled`, whose
   rules a core release can change, as #115 did), and `provider/httpserver` and
   `provider/metrics` (`tls.ServerConfig`, for the same reason) — to `vX.Y.Z`:
   `make update-deps VERSION=vX.Y.Z`, or `cd provider/<name> && go get
   github.com/oddbit-project/blueprint@vX.Y.Z && go mod tidy` per module. Commit the `go.mod`/
   `go.sum` changes.
4. **Tag the providers** (`make tag-version VERSION=vX.Y.Z` is safe now that step 3 has landed,
   or tag them individually).

`sqlite` is included in the Makefile's `PROVIDERS` list, so `tag-version`, `sbom`,
`update-deps`, `build-providers` and `tidy-providers` already act on it along with the other
provider modules — no separate step is needed for `sqlite` itself.

## Version Strategy

### Semantic Versioning

All modules follow [Semantic Versioning](https://semver.org/):

- **MAJOR.MINOR.PATCH** (e.g., `v0.8.1`)
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Version Coordination

| Change Type       | Core Version          | Provider Action           | Example                 |
|-------------------|-----------------------|---------------------------|-------------------------|
| Provider bug fix  | No change             | Tag provider only         | `provider/redis/v0.8.1` |
| Provider feature  | No change             | Tag provider only         | `provider/s3/v0.8.2`    |
| Core bug fix      | Increment patch       | Optional provider update  | `v0.8.1`                |
| Core feature      | Increment minor       | Optional provider update  | `v0.9.0`                |
| Interface changes | Increment major/minor | Update affected providers | `v1.0.0`                |
| Major release     | Increment major       | Update all providers      | `v1.0.0`                |

## User Installation Examples

### Old Installation (Backward Compatible)

```bash
# Still works - gets everything
go get github.com/oddbit-project/blueprint
```

### Modular Installation (New Way)

```bash
# Install only needed providers
go get github.com/oddbit-project/blueprint/provider/redis@v0.8.1
go get github.com/oddbit-project/blueprint/provider/s3@v0.8.2
go get github.com/oddbit-project/blueprint/provider/httpserver@v0.8.0

# Users can mix versions
go get github.com/oddbit-project/blueprint@v0.8.0              # Core
go get github.com/oddbit-project/blueprint/provider/redis@v0.8.1    # Latest Redis
go get github.com/oddbit-project/blueprint/provider/s3@v0.8.0       # Stable S3
```

## Release Checklist

### Pre-Release

- [ ] All tests passing (`make test-all`)
- [ ] All modules build (`make build-all`)
- [ ] Dependencies tidied (`make tidy-all`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Cross-module dependencies resolved

### Release Process

- [ ] Tags created and pushed
- [ ] GitHub release created with release notes
- [ ] SBOM generated and attached (`make sbom`)
- [ ] Release announcements prepared

### Post-Release

- [ ] Test installation from public repositories
- [ ] Update examples and samples
- [ ] Notify community of changes
- [ ] Monitor for issues

## GitHub Release Notes Template

### For Provider Releases

```markdown
## Redis Provider v0.8.1

### Fixed

- Fixed connection timeout issue in high-concurrency scenarios
- Resolved memory leak in TTL management

### Installation

```bash
go get github.com/oddbit-project/blueprint/provider/redis@v0.8.1
```

**Core Compatibility**: Works with Blueprint core v0.8.0+

```

### For Coordinated Releases

```markdown
## Blueprint v0.9.0

### Breaking Changes
- Updated minimum Go version to 1.21
- Renamed `Config` interface method `GetKey` to `Get`

### Added
- New encryption provider with AES-256-GCM support
- Enhanced logging with structured output
- Rate limiting middleware for HTTP server

### Changed
- Improved error handling across all providers
- Updated all dependencies to latest versions

### Fixed
- Fixed race condition in connection pooling
- Resolved deadlock in transaction handling

### Installation
```bash
# Full framework
go get github.com/oddbit-project/blueprint@v0.9.0

# Or individual providers
go get github.com/oddbit-project/blueprint/provider/redis@v0.9.0
```

```

## Troubleshooting

### Common Issues

1. **Cross-module dependency errors during development**
   - Use the workspace: `GOWORK=/path/to/blueprint/go.work go mod tidy`
   - Check replace directives in go.mod files

2. **Tags not recognized**
   - Ensure tags are pushed to remote: `git push origin --tags`
   - Wait for Go proxy cache refresh (up to 10 minutes)

3. **Version conflicts**
   - Check all go.mod files for consistent versions
   - Use `make update-deps` to synchronize versions

### Development vs Production

**Development** (using workspace):
```bash
GOWORK=/path/to/blueprint/go.work go build
```

**Production** (using published modules):

```bash
go get github.com/oddbit-project/blueprint/provider/redis@v0.8.1
```

## Best Practices

1. **Test thoroughly** before releasing
2. **Keep providers focused** - one responsibility per provider
3. **Maintain backward compatibility** when possible
4. **Document breaking changes** clearly
5. **Use semantic versioning** consistently
6. **Coordinate breaking changes** across dependent modules
7. **Release security fixes quickly** as patch versions

## Contributing

For contributors working on releases:

1. **Follow the branching model** (main/develop/feature branches)
2. **Test cross-module changes** thoroughly
3. **Update documentation** with code changes
4. **Add changelog entries** for user-facing changes
5. **Coordinate with maintainers** for major releases

---
