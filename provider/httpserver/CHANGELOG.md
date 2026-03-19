# HTTP Server Provider Changelog

All notable changes to the Blueprint HTTP Server provider will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.9.1]

### Added

- `FieldValidationError(c, field, message)` helper to build and send a single-field validation error response matching the same format as `ValidateJSON` failures.

### Fixed

- **HMAC type assertion panic**: `GetHMACIdentity()` and `GetHMACDetails()` now use comma-ok type assertions instead of bare casts, preventing panics on unexpected context value types.
- **CSRF token not seeded on first request**: `CSRFProtection()` now generates and stores a `_csrf` token in the session on GET/HEAD/OPTIONS if one does not exist, so the first POST can succeed.
- **Double abort in AuthMiddleware**: removed redundant `c.Abort()` call after `response.Http401()` which already calls `c.AbortWithStatusJSON()`.
- **Fingerprint not stored on first visit**: `FingerprintMiddleware` now generates and stores a fingerprint when none exists, instead of being a no-op until someone else stores one.
- **Rate limiter exceeds maxClients**: when evicting expired entries frees no space, the oldest entry is now evicted to stay within the configured cap.
- **Session save error silently ignored**: post-request `m.store.Set()` errors in session middleware are now logged.
- **auth/session.go MustGet panic**: replaced `c.MustGet()` with safe `c.Get()` + type assertion in `authSession.CanAccess()`.
- **Port validation error message**: corrected from "less than 65535" to "at most 65535".
- **Deprecated Feature-Policy header**: `SecurityMiddleware` now only sets `Permissions-Policy`, no longer emits the deprecated `Feature-Policy` header.
- **JSON marshaller comments**: fixed copy-paste comments on `jsonMarshaller` methods that incorrectly said "use gob".

## [v0.9.0]

### Breaking Changes

- **`ServerConfig.Options` field removed** along with `GetOption()` method and `OptAuthTokenHeader`, `OptAuthTokenSecret`, `OptDefaultSecurityHeaders`, `OptHMACSecret` constants. Use `ServerConfig.ServerName` for server name, and `WithAuthToken()`/`WithDefaultSecurityHeaders()` via `ProcessOptions()` for middleware setup.
- **`RateLimitMiddleware()` return type changed** from `gin.HandlerFunc` to `(gin.HandlerFunc, *ClientRateLimiter)` for lifecycle management.
- **`SecurityConfig.EnableRateLimit` and `SecurityConfig.RateLimit` fields removed**. These were never read by `SecurityMiddleware`; rate limiting is handled independently by `RateLimitMiddleware`.
- **Default server timeouts changed** from 600s to 30s read / 60s write.
- **`init()` functions removed** from `validation.go`, `session/marshaller.go`, and `fingerprint/middleware.go`. Registration is now lazy via `sync.Once`. Only affects code that imports packages purely for side effects without calling their API.

### Added

- `AuthIdentity` struct and `GetAuthIdentity()` for unified identity retrieval across all auth providers (JWT, HMAC, basic auth, session, token). All providers now store identity in context under `ContextAuthIdentity`.
- `WithAuthToken(headerName, secret)` and `WithDefaultSecurityHeaders()` typed `OptionsFunc` constructors replacing string-based options.
- `ServerConfig.ServerName` field replacing `Options["serverName"]`.
- `SessionStore` interface in session package, enabling custom session store implementations (Redis, SQL, etc.). `ManagerWithStore()` now accepts `SessionStore` instead of `*Store`; existing `*Store` satisfies the interface.
- `FingerprintStore` interface and `FingerprintMiddleware()` for storage-agnostic fingerprint validation. `SessionFingerprintStore` provided as default implementation.
- `RegisterGobTypes()` exported functions in session and fingerprint packages for explicit gob type registration.
- `registerValidators()` lazy registration for Gin custom validators via `sync.Once`.
- `UseRateLimiting()` now returns `*security.ClientRateLimiter` for lifecycle management (call `Stop()` on shutdown).
- Context key constants: `log.ContextTraceID`, `log.ContextRequestID`, `security.ContextCSPNonce`, replacing bare string literals.

### Fixed

- Rate limiter IP spoofing: replaced manual `X-Forwarded-For` parsing with `c.ClientIP()` which respects Gin's trusted proxies configuration.
- Rate limiter unbounded memory growth: added `clientEntry` tracking with `lastAccess`, selective eviction, `DefaultMaxClients` cap (10000), and `Stop()` method.
- Session cookie `SameSite` attribute: replaced string concatenation with `c.SetSameSite()` called before `c.SetCookie()`.
- Session `NewManager` nil-config bug: fixed assignment to local variable instead of `manager.config`.
- Fingerprint `GetFingerprint` now reads from session data instead of gin context.
- Fingerprint `Generate()` now uses the configured `geoResolver` instead of hardcoded `getCountryFromIP`.
- Logger middleware now captures return value of immutable builder chain (`WithTraceID`/`WithField`).
- `validateNested` no longer double-calls `Validate()` on structs with pointer-receiver methods.
- `getCountryFromIP` rewritten to use `net.IP.IsPrivate()`/`IsLoopback()`/`IsLinkLocalUnicast()`/`IsLinkLocalMulticast()` covering all RFC private ranges.
- `CorsConfig` struct tag typo fixed (`json:"corsEnabled""` to `json:"corsEnabled"`).

## [v0.8.6]

### Fixed

- Fixed CSRF token comparison to use constant-time comparison (`crypto/subtle`) preventing timing attacks
- CSP nonce generation now uses `crypto/rand` instead of UUID for proper opaque nonces per W3C spec
- Fixed potential deadlock in session `StopCleanup()` when cleanup goroutine is mid-operation
- Fixed swallowed encryption error in session `Set()` that could silently store unencrypted session data

### Security

- CSRF token validation hardened against timing attacks
- CSP nonce generation improved to use cryptographically random bytes
- Session encryption errors now fail explicitly instead of falling back to plaintext storage

## [v0.8.5]

### Changed
- Fixed issue with `ValidateJSON` where json fields were being named using the struct name, instead of the json tag name


## [v0.8.4]

### Added
- `BasicAuthProvider` basic auth provider wih pluggable backend
- `HtpasswdBackend` htpasswd backend for basic auth provider

### Changed
- Refactored `authToken` to use subtle.ConstantTimeCompare()
- Refactored `authTokenList` to use subtle.ConstantTimeCompare()
- Behavior change - to disable auth in authTokenList, an explicit empty string `""` is required as key; 

## [v0.8.3]

### Added
- `Validator` interface for custom validation logic on request structs
- `NewFieldError` helper function for creating field-specific validation errors
- Two-stage validation system: binding validation followed by recursive custom validation
- Support for nested struct validation with full field path reporting (e.g., `address.zip_code`)
- Support for validating slice, array, and map elements
- Comprehensive validation test suite with 1000+ lines covering all validation scenarios

### Changed
- Refactored `ValidateJSON` to support recursive validation of nested structures
- Refactored `ValidateQuery` to support two-stage validation (binding + custom) with recursive nested validation
- Improved validation error handling with better field path context
- Moved from global validator instance to Gin's binding validator
- Enhanced error messages for nested validation failures


## [v0.8.2]

### Added
- TrustedProxies config parameter
- ServerConfig GetUrl() helper

### Changed 
- Fingerprint ip detection changed from custom function to gin-tonic function

## [v0.8.1]

### Added
- Marshaller interface for httpserver.session
- Updated documentation

## [v0.8.0]

### Added
- Initial release of HTTP Server provider as independent module
- Complete HTTP server implementation with security features
- Unified authentication middleware (JWT, HMAC, session)
- mTLS support with client certificate validation
- CORS middleware with configurable policies
- CSRF protection with enhanced token handling
- Rate limiting for DoS protection
- Browser fingerprinting middleware
- Security headers middleware
- Request logging and metrics integration
- Configuration management with TLS/SSL
- Sample applications with authentication examples
- Comprehensive error handling

### Technical Details
- HTTP server implementation with graceful shutdown
- Middleware chain architecture
- Session management with multiple backends
- Certificate-based authentication
- Connection pooling and keep-alive support
- Request timeout and size limit handling

### Dependencies
- Compatible with Blueprint core framework v0.8.0+
- Optional integration with JWT, HMAC, and session providers

### Migration Notes
- Enhanced security response patterns
- Improved middleware integration
- No breaking changes from previous Blueprint versions
- All existing imports continue to work unchanged