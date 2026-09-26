package sqlb

import (
	"math"
	"regexp"
	"sort"
	"strconv"
)

// kv is a single SETTINGS entry.
type kv struct {
	key   string
	value any
}

var settingsKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Final adds FINAL to the FROM clause. ErrUnsupported unless the dialect
// has FeatureClickHouse, or the FROM source is a Subquery.
func (s *SelectBuilder) Final() *SelectBuilder {
	c := s.clone()
	c.final = true
	return c
}

// Sample adds SAMPLE ratio to the FROM clause. ratio must be in (0, 1] and
// finite, else ErrInvalidSample at Build. ErrUnsupported unless the
// dialect has FeatureClickHouse, or the FROM source is a Subquery.
func (s *SelectBuilder) Sample(ratio float64) *SelectBuilder {
	c := s.clone()
	c.hasSample = true
	c.sampleIsRows = false
	c.sampleRatio = ratio
	return c
}

// SampleRows adds SAMPLE n (an absolute row count) to the FROM clause. n
// must be >= 1, else ErrInvalidSample at Build. ErrUnsupported unless the
// dialect has FeatureClickHouse, or the FROM source is a Subquery.
func (s *SelectBuilder) SampleRows(n uint64) *SelectBuilder {
	c := s.clone()
	c.hasSample = true
	c.sampleIsRows = true
	c.sampleRows = n
	return c
}

// ArrayJoin adds an ARRAY JOIN clause over cols (by position; Col("a").
// As("x") allowed). Zero columns fails with ErrEmptyClause; calling both
// ArrayJoin and LeftArrayJoin on the same builder fails with
// ErrUnsupported. ErrUnsupported unless the dialect has FeatureClickHouse.
func (s *SelectBuilder) ArrayJoin(cols ...any) *SelectBuilder {
	c := s.clone()
	c.hasArrayJoin = true
	c.arrayJoinLeft = false
	c.arrayJoin = append([]any(nil), cols...)
	c.arrayJoinCalls = s.arrayJoinCalls + 1
	return c
}

// LeftArrayJoin adds a LEFT ARRAY JOIN clause over cols. Zero columns
// fails with ErrEmptyClause; calling both ArrayJoin and LeftArrayJoin on
// the same builder fails with ErrUnsupported. ErrUnsupported unless the
// dialect has FeatureClickHouse.
func (s *SelectBuilder) LeftArrayJoin(cols ...any) *SelectBuilder {
	c := s.clone()
	c.hasArrayJoin = true
	c.arrayJoinLeft = true
	c.arrayJoin = append([]any(nil), cols...)
	c.arrayJoinCalls = s.arrayJoinCalls + 1
	return c
}

// Prewhere appends conds, ANDed with any existing PREWHERE conditions.
// ErrUnsupported unless the dialect has FeatureClickHouse.
func (s *SelectBuilder) Prewhere(conds ...Expr) *SelectBuilder {
	c := s.clone()
	c.prewhere = append(c.prewhere, conds...)
	return c
}

// Settings merges m into the SETTINGS clause: existing keys are
// overwritten, new keys are added. m is copied into a sorted []kv;
// mutating m after the call has no effect. Keys must match
// ^[A-Za-z_][A-Za-z0-9_]*$, else ErrInvalidSetting at Build.
// ErrUnsupported unless the dialect has FeatureClickHouse.
func (s *SelectBuilder) Settings(m map[string]any) *SelectBuilder {
	c := s.clone()
	merged := append([]kv(nil), s.settings...)
	for k, v := range m {
		found := false
		for i := range merged {
			if merged[i].key == k {
				merged[i].value = v
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, kv{key: k, value: v})
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].key < merged[j].key })
	c.settings = merged
	return c
}

// renderFinalSample renders " FINAL" and/or " SAMPLE <v>" after the FROM
// source, if set.
func (s *SelectBuilder) renderFinalSample(w *writer) {
	if !s.final && !s.hasSample {
		return
	}
	if !w.d.Has(FeatureClickHouse) {
		w.fail(ErrUnsupported)
		return
	}
	if _, ok := s.from.(Subquery); ok {
		w.fail(ErrUnsupported)
		return
	}
	if s.final {
		w.keyword(" FINAL")
	}
	if s.hasSample {
		if s.sampleIsRows {
			if s.sampleRows < 1 {
				w.fail(ErrInvalidSample)
				return
			}
			w.keyword(" SAMPLE " + strconv.FormatUint(s.sampleRows, 10))
			return
		}
		r := s.sampleRatio
		if math.IsNaN(r) || math.IsInf(r, 0) || r <= 0 || r > 1 {
			w.fail(ErrInvalidSample)
			return
		}
		w.keyword(" SAMPLE " + strconv.FormatFloat(r, 'f', -1, 64))
	}
}

// renderArrayJoin renders " ARRAY JOIN <cols>" or " LEFT ARRAY JOIN
// <cols>", if set.
func (s *SelectBuilder) renderArrayJoin(w *writer) {
	if !s.hasArrayJoin {
		return
	}
	if !w.d.Has(FeatureClickHouse) {
		w.fail(ErrUnsupported)
		return
	}
	if s.arrayJoinCalls > 1 {
		w.fail(ErrUnsupported)
		return
	}
	if len(s.arrayJoin) == 0 {
		w.fail(ErrEmptyClause)
		return
	}
	if s.arrayJoinLeft {
		w.keyword(" LEFT ARRAY JOIN ")
	} else {
		w.keyword(" ARRAY JOIN ")
	}
	for i, c := range s.arrayJoin {
		if i > 0 {
			w.keyword(", ")
		}
		renderColumn(w, c)
	}
}

// renderSettings renders " SETTINGS k = v, ...", if set. Keys are written
// unquoted; values are bound with arg.
func (s *SelectBuilder) renderSettings(w *writer) {
	if len(s.settings) == 0 {
		return
	}
	if !w.d.Has(FeatureClickHouse) {
		w.fail(ErrUnsupported)
		return
	}
	w.keyword(" SETTINGS ")
	for i, e := range s.settings {
		if !settingsKeyRe.MatchString(e.key) {
			w.fail(ErrInvalidSetting)
			return
		}
		if i > 0 {
			w.keyword(", ")
		}
		w.keyword(e.key + " = ")
		w.arg(e.value)
	}
}
