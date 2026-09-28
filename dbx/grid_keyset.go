package dbx

import (
	"context"
	"fmt"
	"math"
	"reflect"

	"github.com/oddbit-project/gohan"
)

// QueryGridKeyset is QueryGrid with keyset (cursor) pagination: it returns
// one page of the rows q's filters and search match, in QueryGrid's order
// (q's sort fields, then g's WithTiebreaker columns), starting after cursor
// ("" for the first page). q.Limit is the page size, capped as QueryGrid
// caps it (DefaultPageSize when the grid has no cap and q.Limit is 0).
//
// g must have a tiebreaker (WithTiebreaker) whose columns identify rows
// uniquely, else ErrInvalidKeysetKey; two fetched rows with the same key
// values fail with ErrKeysetNotUnique. Every key must be of a Go type
// ListKeyset accepts: a client sort field that is not fails with a
// GridError (scope "sort"), a tiebreaker that is not with ErrInvalidKeysetKey.
// q.Offset must be 0, at most MaxKeysetKeys keys may apply, and the page
// size must be below math.MaxInt64 (reachable only with WithMaxLimit(0)),
// else a GridError. q is validated as QueryGrid validates it, and a cursor
// that is malformed or was minted for another sort fails with a GridError
// of scope "cursor" (errors.Is(err, ErrInvalidCursor)); all before any
// query. The cursor survives a change of filters or search. A next-page
// cursor longer than MaxCursorBytes fails the page with ErrCursorTooLarge,
// and a string key that is not valid UTF-8 with ErrInvalidKeysetKey: the
// client picks the sort, so bound the length of every sortable string
// column.
func (r *Repository[T]) QueryGridKeyset(ctx context.Context, g *Grid[T], q *GridQuery, cursor string) (*KeysetPage[T], error) {
	if q == nil {
		return nil, GridError{Scope: "query", Message: "query is required"}
	}
	if err := g.configErr(); err != nil {
		return nil, err
	}
	if q.Offset != 0 {
		return nil, GridError{Scope: "query", Message: "offset is not allowed with cursor pagination"}
	}
	conds, err := g.Conds(q)
	if err != nil {
		return nil, err
	}

	if len(g.tiebreaker) == 0 {
		return nil, fmt.Errorf("%w: grid has no tiebreaker; call WithTiebreaker with a unique key", ErrInvalidKeysetKey)
	}
	fields, err := keysetFieldsFor(reflect.TypeFor[T]())
	if err != nil {
		return nil, err
	}
	gkeys := g.orderKeys(q)
	keys := make([]KeysetKey, len(gkeys))
	for i, k := range gkeys {
		if _, err := keysetColumn(fields, k.KeysetKey); err != nil {
			if k.alias != "" {
				return nil, GridError{Scope: "sort", Field: k.alias, Message: "field cannot be used with cursor pagination"}
			}
			return nil, err
		}
		keys[i] = k.KeysetKey
	}
	if len(keys) > MaxKeysetKeys {
		return nil, GridError{Scope: "sort", Message: "too many sort fields for cursor pagination"}
	}
	p, err := newKeysetPlan(fields, r.table, keys, r.q.Dialect().Has(gohan.FeatureClickHouse))
	if err != nil {
		return nil, err
	}

	limit := q.Limit
	if g.maxLimit > 0 && (limit == 0 || limit > g.maxLimit) {
		limit = g.maxLimit
	}
	if limit == 0 {
		limit = DefaultPageSize
	}
	// the page is fetched with LIMIT limit+1, which must fit an int64.
	if uint64(limit) >= math.MaxInt64 {
		return nil, GridError{Scope: "query", Message: "limit is out of range"}
	}

	after, err := p.decode(cursor)
	if err != nil {
		return nil, GridError{Scope: "cursor", Message: "cursor is not valid"}
	}

	sel := r.sel
	if len(conds) > 0 {
		sel = sel.Where(conds...)
	}
	return r.keysetPage(ctx, sel, p, int(limit), after)
}
