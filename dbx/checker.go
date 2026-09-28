package dbx

import "reflect"

// RecordChecker is implemented by a Querier that scans rows into (or reads
// rows from) a record through its own field-mapping rules rather than the
// `db` tag rules gohan builds column lists from — for example
// provider/clickhouse's Querier, which goes through clickhouse-go's
// `ch`-tag struct mapper. NewRepository calls CheckRecord with the record
// type once its columns are resolved and fails with the returned error, so
// a record whose columns the Querier would map to different fields is
// rejected at setup instead of silently scanning into the wrong fields.
type RecordChecker interface {
	// CheckRecord returns a non-nil error if any column gohan.RecordColumns
	// reports for t (a struct type) would not map to the same field under
	// the Querier's own rules.
	CheckRecord(t reflect.Type) error
}
