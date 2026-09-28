package dbx

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPlan builds a keyset plan over ksEligible's fields for table.
func testPlan(t testing.TB, table string, clickhouse bool, keys ...KeysetKey) *keysetPlan {
	t.Helper()
	fields, err := keysetFieldsFor(reflect.TypeFor[ksEligible]())
	require.NoError(t, err)
	p, err := newKeysetPlan(fields, table, keys, clickhouse)
	require.NoError(t, err)
	return p
}

// forge builds a well-formed cursor for p carrying the raw key strings keys.
func forge(t testing.TB, p *keysetPlan, keys ...string) string {
	t.Helper()
	b, err := marshalPayload(cursorPayload{V: cursorVersion, F: p.fp, K: keys})
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

// encodeRaw base64url-encodes an arbitrary payload.
func encodeRaw(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

// payloadOf decodes a cursor's JSON payload.
func payloadOf(t *testing.T, c string) string {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(c)
	require.NoError(t, err)
	return string(b)
}

func roundTrip(t *testing.T, p *keysetPlan, v any) any {
	t.Helper()
	c, err := p.mint([]any{v})
	require.NoError(t, err)
	got, err := p.decode(c)
	require.NoError(t, err)
	require.Len(t, got, 1)
	return got[0]
}

// --- (1) round trip per kind ---

func TestCursorRoundTrip(t *testing.T) {
	t.Run("ints", func(t *testing.T) {
		cases := []struct {
			col  string
			vals []int64
		}{
			{"i8", []int64{math.MinInt8, -1, 0, math.MaxInt8}},
			{"i16", []int64{math.MinInt16, -1, 0, math.MaxInt16}},
			{"i32", []int64{math.MinInt32, -1, 0, math.MaxInt32}},
			{"id", []int64{math.MinInt64, -1, 0, math.MaxInt64}},
		}
		for _, c := range cases {
			p := testPlan(t, "t", false, KeyAsc(c.col))
			for _, v := range c.vals {
				assert.Equal(t, v, roundTrip(t, p, v), "%s %d", c.col, v)
			}
		}
	})

	t.Run("uints", func(t *testing.T) {
		p := testPlan(t, "t", false, KeyAsc("u8"))
		assert.Equal(t, uint64(0), roundTrip(t, p, uint64(0)))
		assert.Equal(t, uint64(math.MaxUint8), roundTrip(t, p, uint64(math.MaxUint8)))
		p = testPlan(t, "t", true, KeyAsc("u64"))
		assert.Equal(t, uint64(0), roundTrip(t, p, uint64(0)))
		assert.Equal(t, uint64(math.MaxUint64), roundTrip(t, p, uint64(math.MaxUint64)))
		p = testPlan(t, "t", false, KeyAsc("u64"))
		assert.Equal(t, uint64(math.MaxInt64), roundTrip(t, p, uint64(math.MaxInt64)))
	})

	t.Run("strings", func(t *testing.T) {
		p := testPlan(t, "t", false, KeyAsc("s"))
		for _, s := range []string{"", "plain", "héllo 世界 🙂", `"`, `\`, "a\x00b\x01\x1f\n\t", "<>&", "\u2028\u2029", "\ufffd"} {
			assert.Equal(t, s, roundTrip(t, p, s), "%q", s)
		}
		c, err := p.mint([]any{"<>&"})
		require.NoError(t, err)
		assert.Contains(t, payloadOf(t, c), `"<>&"`, "HTML characters are not escaped")
	})

	t.Run("times", func(t *testing.T) {
		p := testPlan(t, "t", false, KeyAsc("when"))
		cases := []time.Time{
			time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC),
			time.Date(2024, 5, 6, 7, 8, 9, 123456789, time.UTC),
			time.Date(2024, 5, 6, 7, 8, 9, 500000000, time.UTC),
			time.Date(2024, 5, 6, 7, 8, 9, 0, time.FixedZone("x", 5*3600+1800)),
		}
		for _, tm := range cases {
			got := roundTrip(t, p, tm.UTC()).(time.Time)
			assert.True(t, got.Equal(tm), "%v", tm)
			assert.Equal(t, time.UTC, got.Location())
		}
	})

	t.Run("uuids", func(t *testing.T) {
		p := testPlan(t, "t", false, KeyAsc("uid"))
		for _, u := range []uuid.UUID{uuid.Nil, uuid.New(), uuid.Max} {
			assert.Equal(t, u, roundTrip(t, p, u))
		}
	})

	t.Run("several keys", func(t *testing.T) {
		p := testPlan(t, "t", false, KeyDesc("when"), KeyAsc("s"), KeyAsc("id"))
		vals := []any{time.Date(2024, 1, 2, 3, 4, 5, 6000, time.UTC), "x", int64(7)}
		c, err := p.mint(vals)
		require.NoError(t, err)
		got, err := p.decode(c)
		require.NoError(t, err)
		assert.Equal(t, vals, got)
	})

	t.Run("empty cursor is the first page", func(t *testing.T) {
		got, err := testPlan(t, "t", false, KeyAsc("id")).decode("")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

// --- (2) golden wire format ---

func TestCursorWireFormat(t *testing.T) {
	p := testPlan(t, "items", false, KeyDesc("when"), KeyAsc("id"))

	h := fnv.New64a()
	for _, s := range []string{"dbx-keyset", "1", "items", "when", "desc", "time", "id", "asc", "int64"} {
		_, _ = fmt.Fprintf(h, "%d:%s;", len(s), s)
	}
	fp := fmt.Sprintf("%016x", h.Sum64())
	assert.Equal(t, fp, p.fp)

	c, err := p.mint([]any{time.Date(2024, 5, 6, 7, 8, 9, 500000000, time.UTC), int64(42)})
	require.NoError(t, err)
	want := `{"v":1,"f":"` + fp + `","k":["2024-05-06T07:08:09.5Z","42"]}`
	assert.Equal(t, want, payloadOf(t, c))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString([]byte(want)), c)

	u := uuid.MustParse("0190a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b")
	c, err = testPlan(t, "items", false, KeyAsc("uid")).mint([]any{u})
	require.NoError(t, err)
	assert.Contains(t, payloadOf(t, c), `"k":["0190a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b"]`)
}

// --- (3) tampering ---

func TestCursorTampering(t *testing.T) {
	p := testPlan(t, "items", false, KeyDesc("when"), KeyAsc("id"))
	good, err := p.mint([]any{time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC), int64(42)})
	require.NoError(t, err)
	goodJSON := payloadOf(t, good)
	fp := p.fp
	keys := `"k":["2024-05-06T07:08:09Z","42"]`

	// trailing bits: the payload's length is not a multiple of 3, so the
	// last character carries unused low bits, which must be zero.
	require.NotZero(t, len(goodJSON)%3)
	const urlAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(urlAlphabet, good[len(good)-1])
	trailing := good[:len(good)-1] + string(urlAlphabet[last+1])

	cases := map[string]string{
		"bad base64":           "!!!!",
		"padding":              good + "==",
		"std alphabet plus":    "+" + good[1:],
		"std alphabet slash":   "/" + good[1:],
		"trailing bits":        trailing,
		"not JSON":             encodeRaw("hello"),
		"trailing garbage":     encodeRaw(goodJSON + "x"),
		"second value":         encodeRaw(goodJSON + goodJSON),
		"trailing newline":     encodeRaw(goodJSON + "\n"),
		"inner whitespace":     encodeRaw(`{"v":1, "f":"` + fp + `",` + keys + `}`),
		"unknown field":        encodeRaw(`{"v":1,"f":"` + fp + `",` + keys + `,"x":1}`),
		"duplicate field":      encodeRaw(`{"v":1,"v":1,"f":"` + fp + `",` + keys + `}`),
		"reordered fields":     encodeRaw(`{"f":"` + fp + `","v":1,` + keys + `}`),
		"upper-case field":     encodeRaw(`{"V":1,"f":"` + fp + `",` + keys + `}`),
		"null element":         encodeRaw(`{"v":1,"f":"` + fp + `","k":[null,"42"]}`),
		"number element":       encodeRaw(`{"v":1,"f":"` + fp + `","k":["2024-05-06T07:08:09Z",42]}`),
		"null keys":            encodeRaw(`{"v":1,"f":"` + fp + `","k":null}`),
		"escaped character":    encodeRaw(`{"v":1,"f":"` + fp + `","k":["2024-05-06T07:08:09Z","4\u0032"]}`),
		"version as float":     encodeRaw(`{"v":1.0,"f":"` + fp + `",` + keys + `}`),
		"version 0":            encodeRaw(`{"v":0,"f":"` + fp + `",` + keys + `}`),
		"version 2":            encodeRaw(`{"v":2,"f":"` + fp + `",` + keys + `}`),
		"missing version":      encodeRaw(`{"f":"` + fp + `",` + keys + `}`),
		"empty payload":        encodeRaw(`{}`),
		"too few keys":         forge(t, p, "2024-05-06T07:08:09Z"),
		"too many keys":        forge(t, p, "2024-05-06T07:08:09Z", "42", "1"),
		"other table":          mintWith(t, testPlan(t, "other", false, KeyDesc("when"), KeyAsc("id"))),
		"other column":         mintWith(t, testPlan(t, "items", false, KeyDesc("when"), KeyAsc("i"))),
		"other direction":      mintWith(t, testPlan(t, "items", false, KeyAsc("when"), KeyAsc("id"))),
		"other kind":           mintWith(t, testPlan(t, "items", false, KeyDesc("when"), KeyAsc("u64"))),
		"other width":          mintWith(t, testPlan(t, "items", false, KeyDesc("when"), KeyAsc("i32"))),
		"other order":          mintWith(t, testPlan(t, "items", false, KeyAsc("id"), KeyDesc("when"))),
		"one key of two":       mintWith(t, testPlan(t, "items", false, KeyDesc("when"))),
		"fingerprint case":     encodeRaw(strings.Replace(goodJSON, fp, strings.ToUpper(fp), 1)),
		"cursor of other kind": encodeRaw(`{"v":1,"f":"` + fp + `","k":["2024-05-06T07:08:09Z","x"]}`),
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := p.decode(c)
			require.ErrorIs(t, err, ErrInvalidCursor)
			assert.NotContains(t, err.Error(), fp)
			assert.NotContains(t, err.Error(), "items")
			if len(c) > 8 {
				assert.NotContains(t, err.Error(), c)
			}
		})
	}

	t.Run("sanity: the good cursor decodes", func(t *testing.T) {
		_, err := p.decode(good)
		require.NoError(t, err)
	})

	t.Run("character flips never panic", func(t *testing.T) {
		const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_+/= "
		for i := range good {
			for _, ch := range alphabet {
				if byte(ch) == good[i] {
					continue
				}
				c := good[:i] + string(ch) + good[i+1:]
				assert.NotPanics(t, func() {
					if vals, err := p.decode(c); err == nil {
						again, err := p.mint(vals)
						require.NoError(t, err)
						assert.Equal(t, c, again, "a decodable cursor is canonical")
					}
				})
			}
		}
	})
}

func mintWith(t *testing.T, p *keysetPlan) string {
	t.Helper()
	vals := make([]any, len(p.keys))
	for i, k := range p.keys {
		switch k.typ.kind {
		case keyInt:
			vals[i] = int64(42)
		case keyUint:
			vals[i] = uint64(42)
		case keyTime:
			vals[i] = time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
		default:
			t.Fatalf("unexpected kind %v", k.typ)
		}
	}
	c, err := p.mint(vals)
	require.NoError(t, err)
	return c
}

// --- (4) size cap ---

func TestCursorSizeCap(t *testing.T) {
	p := testPlan(t, "t", false, KeyAsc("s"))

	t.Run("over the cap is rejected before decoding", func(t *testing.T) {
		_, err := p.decode(strings.Repeat("!", MaxCursorBytes+1))
		require.ErrorIs(t, err, ErrInvalidCursor)
		assert.Contains(t, err.Error(), "too large")
	})

	// payload {"v":1,"f":"<16>","k":["<s>"]} is 39+len(s) bytes; 3072 bytes
	// encode to exactly 4096 characters.
	atCap := strings.Repeat("a", 3072-39)

	t.Run("exactly the cap is accepted", func(t *testing.T) {
		c, err := p.mint([]any{atCap})
		require.NoError(t, err)
		require.Len(t, c, MaxCursorBytes)
		got, err := p.decode(c)
		require.NoError(t, err)
		assert.Equal(t, []any{atCap}, got)
	})

	t.Run("minting over the cap is a server error", func(t *testing.T) {
		_, err := p.mint([]any{atCap + "a"})
		require.ErrorIs(t, err, ErrCursorTooLarge)
		assert.NotErrorIs(t, err, ErrInvalidCursor)
		_, err = p.mint([]any{strings.Repeat("é", 2000)})
		require.ErrorIs(t, err, ErrCursorTooLarge)
	})

	t.Run("minting an invalid UTF-8 string key", func(t *testing.T) {
		_, err := p.mint([]any{"a\xffb"})
		require.ErrorIs(t, err, ErrInvalidKeysetKey)
		assert.Contains(t, err.Error(), `"s"`)
	})

	t.Run("minting a time outside years 1-9999", func(t *testing.T) {
		tp := testPlan(t, "t", false, KeyAsc("when"))
		for _, tm := range []time.Time{
			time.Date(0, 12, 31, 23, 59, 59, 0, time.UTC),
			time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		} {
			_, err := tp.mint([]any{tm})
			require.ErrorIs(t, err, ErrInvalidKeysetKey, "%v", tm)
		}
	})
}

// TestCursorMintClickHouseTimeRange checks that on ClickHouse mint rejects,
// as a server error, a time key decode would reject, instead of handing the
// client a cursor its next request fails with as a client error.
func TestCursorMintClickHouseTimeRange(t *testing.T) {
	ch := testPlan(t, "t", true, KeyAsc("when"))
	for _, tm := range []time.Time{
		clickHouseMinTime.Add(-time.Nanosecond),
		clickHouseMaxTime.Add(time.Nanosecond),
		time.Date(2299, 12, 31, 23, 59, 59, 0, time.UTC),
	} {
		_, err := ch.mint([]any{tm})
		require.ErrorIs(t, err, ErrInvalidKeysetKey, "%v", tm)
		assert.NotErrorIs(t, err, ErrInvalidCursor)
		assert.Contains(t, err.Error(), `"when"`)
	}
	for _, tm := range []time.Time{clickHouseMinTime, clickHouseMaxTime} {
		assert.True(t, tm.Equal(roundTrip(t, ch, tm).(time.Time)), "%v", tm)
	}

	other := testPlan(t, "t", false, KeyAsc("when"))
	for _, tm := range []time.Time{clickHouseMinTime.Add(-time.Nanosecond), clickHouseMaxTime.Add(time.Nanosecond)} {
		assert.True(t, tm.Equal(roundTrip(t, other, tm).(time.Time)), "%v", tm)
	}
}

// --- (5) range and canonical checks per kind ---

func TestCursorKeyChecks(t *testing.T) {
	cases := []struct {
		col        string
		clickhouse bool
		ok         []string
		bad        []string
	}{
		{"i8", false, []string{"127", "-128", "0", "-1"}, []string{"128", "-129", "+1", "01", "-0", " 1", "1 ", "1e3", "0x10", "1_0", "", "1.0"}},
		{"id", false, []string{"9223372036854775807", "-9223372036854775808"}, []string{"9223372036854775808", "-9223372036854775809", "+1", "01", "-0", " 1", "1e3", "0x10"}},
		{"u8", false, []string{"0", "255"}, []string{"256", "-1", "+1", "01", "-0", "0x10"}},
		{"u64", true, []string{"0", "18446744073709551615", "9223372036854775808"}, []string{"18446744073709551616", "-1", "01"}},
		{"u64", false, []string{"0", "9223372036854775807"}, []string{"9223372036854775808", "18446744073709551615"}},
		{"u", false, []string{"9223372036854775807"}, []string{"9223372036854775808"}},
		{"when", false,
			[]string{"2024-01-02T03:04:05Z", "2024-01-02T03:04:05.5Z", "2024-01-02T03:04:05.123456789Z", "0001-01-01T00:00:00Z", "9999-12-31T23:59:59.999999999Z"},
			[]string{"2024-01-02T03:04:05+01:00", "2024-01-02T03:04:05+00:00", "2024-01-02T03:04:05.500Z", "2024-01-02T03:04:05.0Z",
				"2024-01-02T03:04:05z", "2024-01-02t03:04:05Z", "2024-01-02", "2024-01-02 03:04:05Z", "0000-01-01T00:00:00Z", "2024-13-01T00:00:00Z", "2024-01-02T03:04:05.1234567891Z", ""}},
		{"when", true,
			[]string{"1900-01-01T00:00:00Z", "2262-04-11T23:47:16.854775807Z", "2024-01-02T03:04:05.5Z"},
			[]string{"1899-12-31T23:59:59.999999999Z", "0001-01-01T00:00:00Z", "2262-04-11T23:47:16.854775808Z",
				"2299-12-31T23:59:59.999999999Z", "2300-01-01T00:00:00Z", "9999-12-31T23:59:59.999999999Z"}},
		{"when", false, []string{"1899-12-31T23:59:59.999999999Z", "2300-01-01T00:00:00Z"}, nil},
		{"uid", false,
			[]string{"0190a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b", "00000000-0000-0000-0000-000000000000"},
			[]string{"0190A0B1-C2D3-7E4F-8A9B-0C1D2E3F4A5B", "{0190a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b}", "0190a0b1c2d37e4f8a9b0c1d2e3f4a5b",
				"urn:uuid:0190a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5b", "0190a0b1-c2d3-7e4f-8a9b-0c1d2e3f4a5", ""}},
		{"s", false, []string{"", " 01 ", "anything"}, nil},
	}
	for _, c := range cases {
		p := testPlan(t, "t", c.clickhouse, KeyAsc(c.col))
		for _, s := range c.ok {
			t.Run(c.col+"/"+strconv.FormatBool(c.clickhouse)+"/ok/"+s, func(t *testing.T) {
				vals, err := p.decode(forge(t, p, s))
				require.NoError(t, err)
				again, err := p.mint(vals)
				require.NoError(t, err)
				assert.Equal(t, forge(t, p, s), again)
			})
		}
		for _, s := range c.bad {
			t.Run(c.col+"/"+strconv.FormatBool(c.clickhouse)+"/bad/"+s, func(t *testing.T) {
				_, err := p.decode(forge(t, p, s))
				require.ErrorIs(t, err, ErrInvalidCursor)
				assert.Contains(t, err.Error(), "key 0 out of range")
			})
		}
	}
}

// --- (6) fuzz ---

func FuzzDecodeCursor(f *testing.F) {
	p := testPlan(f, "items", false, KeyDesc("when"), KeyAsc("s"), KeyAsc("uid"), KeyAsc("i8"), KeyAsc("u64"))
	seeds := [][]any{
		{time.Date(2024, 5, 6, 7, 8, 9, 5, time.UTC), "x", uuid.Nil, int64(-1), uint64(1)},
		{time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), "", uuid.Max, int64(127), uint64(math.MaxInt64)},
		{time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), "\"\\<>\u2028", uuid.New(), int64(-128), uint64(0)},
	}
	for _, vals := range seeds {
		c, err := p.mint(vals)
		require.NoError(f, err)
		f.Add(c)
	}
	f.Add("")
	f.Add("e30")
	f.Fuzz(func(t *testing.T, c string) {
		vals, err := p.decode(c)
		if err != nil {
			require.ErrorIs(t, err, ErrInvalidCursor)
			return
		}
		if c == "" {
			return
		}
		again, err := p.mint(vals)
		require.NoError(t, err)
		require.Equal(t, c, again, "a decodable cursor is canonical")
	})
}
