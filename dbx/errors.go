package dbx

import (
	"database/sql"

	"github.com/oddbit-project/blueprint/utils"
)

// ErrNotFound is returned when a query expected to match a row matches
// none. It is an alias for sql.ErrNoRows so errors.Is(err, sql.ErrNoRows)
// keeps working.
var ErrNotFound = sql.ErrNoRows

const (
	// ErrNotStruct is returned by NewRepository when T is not a struct type.
	ErrNotStruct = utils.Error("dbx: repository type must be a struct")
	// ErrUnknownColumn is returned when a caller-supplied field/column name
	// is not one of the repository's record columns.
	ErrUnknownColumn = utils.Error("dbx: column is not a field of the record type")
	// ErrTxUnsupported is returned by WithTx when the given Querier is
	// neither a TxQuerier nor a TxBeginner.
	ErrTxUnsupported = utils.Error("dbx: querier cannot begin a transaction")
	// ErrCountOverflow is returned when a COUNT result does not fit in
	// int64.
	ErrCountOverflow = utils.Error("dbx: count does not fit in int64")
	// ErrNoColumns is returned by NewRepository when the record type maps
	// to zero columns.
	ErrNoColumns = utils.Error("dbx: record type has no columns")
	// ErrDialectDriver is returned by FromClient when the client's driver
	// resolves to a dialect the database/sql adapter does not support
	// (currently ClickHouse; use provider/clickhouse's Querier instead).
	ErrDialectDriver = utils.Error("dbx: driver/dialect not supported by the database/sql adapter")
)
