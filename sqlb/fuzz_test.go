package sqlb

import (
	"strings"
	"testing"
)

// unquoteIdent scans out to find the first unescaped closing quote and
// returns the un-escaped body. This is an independent, test-local
// tokenizer: it does not reuse the ident escaping rules.
func unquoteIdent(t *testing.T, out string, quote byte) (body string, ok bool) {
	t.Helper()
	if len(out) < 2 || out[0] != quote {
		return "", false
	}
	i := 1
	var b strings.Builder
	for i < len(out) {
		c := out[i]
		if c == quote {
			// Escaped quote: doubled.
			if i+1 < len(out) && out[i+1] == quote {
				b.WriteByte(quote)
				i += 2
				continue
			}
			// Unescaped closing quote: must be the last character.
			if i != len(out)-1 {
				return "", false
			}
			return b.String(), true
		}
		b.WriteByte(c)
		i++
	}
	return "", false
}

// unescapeClickHouseBackslash reverses the ClickHouse backslash-doubling
// escape applied before quote-doubling.
func unescapeBackslash(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '\\' {
			b.WriteByte('\\')
			i += 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func FuzzIdentRoundTrip(f *testing.F) {
	seeds := []string{"a", `a"b`, "a`b", `c\d`, `"; DROP TABLE t; --`}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if strings.Contains(s, ".") || strings.ContainsRune(s, 0) || s == "" || s == "*" {
			t.Skip()
		}
		for _, d := range []Dialect{Postgres(), SQLite(), ClickHouse(), Generic()} {
			if d.quote == quoteClickHouse && strings.ContainsAny(s, "?@{}") {
				continue
			}
			if d.quote == quoteClickHouse && clickHouseIdentBan.MatchString(s) {
				continue
			}
			out, _, err := render(d, Col(s))
			if err != nil {
				continue
			}
			var quote byte
			if d.quote == quoteBacktick {
				quote = '`'
			} else {
				quote = '"'
			}
			if len(out) == 0 || out[len(out)-1] != quote {
				t.Fatalf("dialect %s: output %q does not end with closing quote", d.Name(), out)
			}
			body, ok := unquoteIdent(t, out, quote)
			if !ok {
				t.Fatalf("dialect %s: could not tokenize output %q", d.Name(), out)
			}
			if d.quote == quoteClickHouse {
				body = unescapeBackslash(body)
			}
			if body != s {
				t.Fatalf("dialect %s: round trip mismatch: got %q want %q (out=%q)", d.Name(), body, s, out)
			}
		}
	})
}

func FuzzValueNeverInlined(f *testing.F) {
	f.Add("")
	f.Add("' OR 1=1 --")
	f.Add(`x\' OR 1=1 --`)
	f.Fuzz(func(t *testing.T, s string) {
		sql, args, err := render(Postgres(), Col("a").Eq(s))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sql != `"a" = $1` {
			t.Fatalf("got sql %q", sql)
		}
		if len(args) != 1 || args[0] != s {
			t.Fatalf("got args %v", args)
		}
	})
}

func FuzzRawNoOrdinal(f *testing.F) {
	f.Add("plain text")
	f.Add("$1")
	f.Add("?1")
	f.Fuzz(func(t *testing.T, s string) {
		sql, _, err := render(SQLite(), Raw(s))
		if err != nil {
			return
		}
		if strings.ContainsAny(sql, "0123456789") {
			for i := 0; i < len(sql); i++ {
				if (sql[i] == '$' || sql[i] == '?') && i+1 < len(sql) && isDigit(sql[i+1]) {
					t.Fatalf("output contains ordinal placeholder: %q", sql)
				}
			}
		}
	})
}
