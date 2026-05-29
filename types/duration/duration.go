// Package duration provides a JSON-friendly duration type for goauth
// configuration. Values are stored as whole seconds (int64) so they
// serialize as a plain integer, matching the OAuth/OIDC `expires_in`
// convention and giving operators a config file they can read at a
// glance.
//
// The canonical type is duration.Seconds (Go-idiomatic stutter, like
// time.Time or url.URL). Construct values with the named helpers,
// which all return Seconds:
//
//	cfg.AccessTokenTTL  = duration.Minutes(15)
//	cfg.RefreshTokenTTL = duration.Days(7)
//	cfg.OIDCStateTTL    = duration.Seconds(600)
//	cfg.JWKSCacheTTL    = duration.FromStd(time.Hour)
//
// Interop with the standard library goes through Std(), which hands
// back a time.Duration:
//
//	timer := time.NewTimer(cfg.AccessTokenTTL.Std())
//	deadline := time.Now().Add(cfg.AccessTokenTTL.Std())
package duration

import "time"

// Seconds is a duration measured in whole seconds. The default
// encoding/json codec serializes it as a plain integer (e.g. 900 for
// fifteen minutes) — no custom marshaler is required because Seconds
// is a defined int64 type.
//
// int64 (not uint64) is the underlying type so subtraction and
// time.Duration interop stay arithmetic-friendly. Validation that a
// TTL must be positive is a separate concern, expressed as `d > 0` at
// the callsite or via the IsPositive method.
type Seconds int64

// Minutes returns the duration equal to n minutes.
func Minutes(n int64) Seconds { return Seconds(n * 60) }

// Hours returns the duration equal to n hours.
func Hours(n int64) Seconds { return Seconds(n * 3600) }

// Days returns the duration equal to n days (24 hours each — no DST,
// no leap seconds; that is a calendar concept, not a duration one).
func Days(n int64) Seconds { return Seconds(n * 86400) }

// FromStd converts a time.Duration to Seconds, truncating toward
// zero. Sub-second precision is lost — that is by design: the public
// surface should not promise precision the JSON form cannot carry.
func FromStd(d time.Duration) Seconds {
	return Seconds(d / time.Second)
}

// Std returns the receiver as a time.Duration for interop with the
// rest of the standard library.
func (d Seconds) Std() time.Duration {
	return time.Duration(d) * time.Second
}

// IsPositive reports whether the duration is strictly greater than
// zero. Useful in Validate() methods that reject non-positive TTLs.
func (d Seconds) IsPositive() bool {
	return d > 0
}

// String formats the duration using time.Duration's String() —
// produces values like "15m0s", "168h0m0s", "30s". Intended for logs
// and error messages; JSON serialization uses the integer form via
// the underlying int64 type.
func (d Seconds) String() string {
	return d.Std().String()
}
