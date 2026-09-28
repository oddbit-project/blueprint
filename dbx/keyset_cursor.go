package dbx

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// cursorVersion is the cursor format version; a format change bumps it and
// invalidates every outstanding cursor.
const cursorVersion = 1

// The time key values a cursor may carry on dialects with
// gohan.FeatureClickHouse: DateTime64's lower bound, and the last instant a
// DateTime64(9) value (or a nanosecond-precision bound time) can hold.
var (
	clickHouseMinTime = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	clickHouseMaxTime = time.Unix(0, math.MaxInt64).UTC()
)

// cursorPayload is a cursor's JSON payload: format version, fingerprint of
// the key list, and one string per key value.
type cursorPayload struct {
	V int      `json:"v"`
	F string   `json:"f"`
	K []string `json:"k"`
}

// marshalPayload encodes p in the one canonical form cursors use (no HTML
// escaping, no trailing newline).
func marshalPayload(p cursorPayload) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// cursorFingerprint identifies the cursor's context: format version, table,
// and each key's column, direction and Go kind. It is FNV-1a 64 over
// length-prefixed fields; it detects a cursor used with another listing, and
// is not a security control.
func cursorFingerprint(table string, keys []keysetCol) string {
	h := fnv.New64a()
	write := func(s string) { _, _ = fmt.Fprintf(h, "%d:%s;", len(s), s) }
	write("dbx-keyset")
	write(strconv.Itoa(cursorVersion))
	write(table)
	for _, k := range keys {
		write(k.col)
		if k.desc {
			write(SortDescending)
		} else {
			write(SortAscending)
		}
		write(k.typ.name())
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// mint encodes the canonical key values vals as a cursor.
func (p *keysetPlan) mint(vals []any) (string, error) {
	keys := make([]string, len(vals))
	for i, v := range vals {
		switch v := v.(type) {
		case int64:
			keys[i] = strconv.FormatInt(v, 10)
		case uint64:
			keys[i] = strconv.FormatUint(v, 10)
		case string:
			if !utf8.ValidString(v) {
				return "", fmt.Errorf("%w: column %q: value is not valid UTF-8", ErrInvalidKeysetKey, p.keys[i].col)
			}
			keys[i] = v
		case time.Time:
			if y := v.Year(); y < 1 || y > 9999 {
				return "", fmt.Errorf("%w: column %q: time is outside years 1 to 9999", ErrInvalidKeysetKey, p.keys[i].col)
			}
			// the range parseKey accepts on ClickHouse
			if p.clickhouse && (v.Before(clickHouseMinTime) || v.After(clickHouseMaxTime)) {
				return "", fmt.Errorf("%w: column %q: time is outside ClickHouse's cursor range", ErrInvalidKeysetKey, p.keys[i].col)
			}
			keys[i] = v.UTC().Format(time.RFC3339Nano)
		case uuid.UUID:
			keys[i] = v.String()
		}
	}
	b, err := marshalPayload(cursorPayload{V: cursorVersion, F: p.fp, K: keys})
	if err != nil {
		return "", err
	}
	c := base64.RawURLEncoding.EncodeToString(b)
	if len(c) > MaxCursorBytes {
		return "", fmt.Errorf("%w: %d bytes", ErrCursorTooLarge, len(c))
	}
	return c, nil
}

// decode checks cursor strictly and returns its canonical key values; "" is
// the first page (nil values). Every failure is ErrInvalidCursor with a fixed
// reason that never echoes the cursor's content.
func (p *keysetPlan) decode(cursor string) ([]any, error) {
	if cursor == "" {
		return nil, nil
	}
	if len(cursor) > MaxCursorBytes {
		return nil, fmt.Errorf("%w: too large", ErrInvalidCursor)
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("%w: bad encoding", ErrInvalidCursor)
	}

	var pl cursorPayload
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&pl); err != nil {
		return nil, fmt.Errorf("%w: bad payload", ErrInvalidCursor)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%w: bad payload", ErrInvalidCursor)
	}
	canon, err := marshalPayload(pl)
	if err != nil || !bytes.Equal(canon, raw) {
		return nil, fmt.Errorf("%w: non-canonical", ErrInvalidCursor)
	}
	if pl.V != cursorVersion {
		return nil, fmt.Errorf("%w: unsupported version", ErrInvalidCursor)
	}
	if pl.F != p.fp {
		return nil, fmt.Errorf("%w: fingerprint mismatch", ErrInvalidCursor)
	}
	if len(pl.K) != len(p.keys) {
		return nil, fmt.Errorf("%w: key count", ErrInvalidCursor)
	}

	vals := make([]any, len(pl.K))
	for i, s := range pl.K {
		v, ok := p.parseKey(p.keys[i].typ, s)
		if !ok {
			return nil, fmt.Errorf("%w: key %d out of range", ErrInvalidCursor, i)
		}
		vals[i] = v
	}
	return vals, nil
}

// parseKey parses one cursor key string of type kt, accepting only the
// exact string mint would produce for the value.
func (p *keysetPlan) parseKey(kt keyType, s string) (any, bool) {
	switch kt.kind {
	case keyInt:
		n, err := strconv.ParseInt(s, 10, kt.bits)
		return n, err == nil && strconv.FormatInt(n, 10) == s
	case keyUint:
		n, err := strconv.ParseUint(s, 10, kt.bits)
		if err != nil || strconv.FormatUint(n, 10) != s {
			return nil, false
		}
		// PostgreSQL and SQLite store signed 64-bit integers at most.
		if !p.clickhouse && n > math.MaxInt64 {
			return nil, false
		}
		return n, true
	case keyString:
		return s, true
	case keyTime:
		if !strings.HasSuffix(s, "Z") {
			return nil, false
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil || t.Year() < 1 || t.Year() > 9999 {
			return nil, false
		}
		t = t.UTC()
		// ClickHouse's DateTime64 range; a bound time outside it fails the
		// query instead of matching no row.
		if p.clickhouse && (t.Before(clickHouseMinTime) || t.After(clickHouseMaxTime)) {
			return nil, false
		}
		return t, t.Format(time.RFC3339Nano) == s
	default:
		u, err := uuid.Parse(s)
		return u, err == nil && u.String() == s
	}
}
