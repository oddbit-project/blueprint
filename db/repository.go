package db

import (
	"context"
	"database/sql"
	"errors"
	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db/qb"
)

type Builder interface {
	Sql() goqu.DialectWrapper
	SqlSelect() *goqu.SelectDataset
	SqlInsert() *goqu.InsertDataset
	SqlUpdate() *goqu.UpdateDataset
	SqlDelete() *goqu.DeleteDataset
}

type SqlBuilder interface {
	SqlDialect() qb.SqlDialect
	SqlBuilder() *qb.SqlBuilder
	SqlUpdateX(record any) *qb.UpdateBuilder
	Do(qry any, target ...any) error
}

type Reader interface {
	FetchOne(qry *goqu.SelectDataset, target any) error
	FetchRecord(fieldValues map[string]any, target any) error
	Fetch(qry *goqu.SelectDataset, target any) error
	FetchWhere(fieldValues map[string]any, target any) error
	FetchByKey(keyField string, value any, target any) error
	Exists(fieldName string, fieldValue any, skip ...any) (bool, error)
}

type Counter interface {
	Count() (int64, error)
	CountWhere(fieldValues map[string]any) (int64, error)
}

type Executor interface {
	Exec(qry *goqu.SelectDataset) error
	RawExec(sql string, args ...any) error
	Select(sql string, target any, args ...any) error
}

type Writer interface {
	Insert(records ...any) error
	InsertReturning(record any, returnFields []string, target ...any) error
}
type Updater interface {
	Update(qry *goqu.UpdateDataset) error
	UpdateReturning(record any, whereFieldsValues map[string]any, returnFields []string, target ...any) error
	UpdateRecord(record any, whereFieldsValues map[string]any) error
	UpdateFields(record any, fieldsValues map[string]any, whereFieldsValues map[string]any) error
	UpdateFieldsReturning(record any, fieldsValues map[string]any, whereFieldsValues map[string]any, returnFields []string, target ...any) error
	UpdateByKey(record any, keyField string, value any) error
}

type Deleter interface {
	Delete(qry *goqu.DeleteDataset) error
	DeleteWhere(fieldNameValue map[string]any) error
	DeleteByKey(keyField string, value any) error
}

type Identifier interface {
	Db() *sqlx.DB
	Name() string
}

type SqlxReaderCtx interface {
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

type GridOps interface {
	Grid(record any) (*Grid, error)
	QueryGrid(record any, args *GridQuery, dest any) error
}

type Dialect interface {
}

type Repository interface {
	Identifier
	Builder
	Reader
	Executor
	Writer
	Deleter
	Updater
	Counter
	GridOps
	SqlBuilder
	NewTransaction(opts *sql.TxOptions) (Transaction, error)
}

type Transaction interface {
	Builder
	Reader
	Executor
	Writer
	Deleter
	Updater
	Counter
	SqlBuilder
	Db() *sqlx.Tx
	Name() string

	Commit() error
	Rollback() error
}

type FV map[string]any // alias for fieldValues maps

type repository struct {
	conn       *sqlx.DB
	ctx        context.Context
	tableName  string
	dialect    goqu.DialectWrapper
	sqlBuilder *qb.SqlBuilder
}

type tx struct {
	conn       *sqlx.Tx
	ctx        context.Context
	tableName  string
	dialect    goqu.DialectWrapper
	sqlBuilder *qb.SqlBuilder
}

func (r *repository) NewTransaction(opts *sql.TxOptions) (Transaction, error) {
	t, err := r.Db().BeginTxx(r.ctx, opts)
	if err != nil {
		return nil, err
	}
	return &tx{
		conn:       t,
		ctx:        r.ctx,
		tableName:  r.tableName,
		dialect:    r.dialect,
		sqlBuilder: r.sqlBuilder,
	}, nil
}

func (r *repository) SqlDialect() qb.SqlDialect {
	return r.sqlBuilder.Dialect()
}

func (r *repository) SqlBuilder() *qb.SqlBuilder {
	return r.sqlBuilder
}

func (r *repository) SqlUpdateX(record any) *qb.UpdateBuilder {
	return r.SqlBuilder().Update(r.tableName, record)
}

func (r *repository) Db() *sqlx.DB {
	return r.conn
}

func (r *repository) Name() string {
	return r.tableName
}

func (r *repository) Sql() goqu.DialectWrapper {
	return r.dialect
}

// SqlSelect returns a Select query builder
func (r *repository) SqlSelect() *goqu.SelectDataset {
	return r.dialect.From(r.tableName)
}

// SqlInsert returns an Insert query builder
// Compatibility note: goqu prepared statements are *not* compatible with postgresql extended
// types; build update clauses with SqlBuilder() instead
func (r *repository) SqlInsert() *goqu.InsertDataset {
	return r.dialect.Insert(r.tableName)
}

// SqlUpdate returns an Update query builder
// Compatibility note: goqu prepared statements are *not* compatible with postgresql extended
// types; build update clauses with SqlBuilder() instead
func (r *repository) SqlUpdate() *goqu.UpdateDataset {
	return r.dialect.Update(r.tableName)
}

// SqlDelete returns a Delete query builder
func (r *repository) SqlDelete() *goqu.DeleteDataset {
	return r.dialect.Delete(r.tableName)
}

// FetchOne fetch a record; target must be a struct
// Example:
//
//		wallets[i] = record
//	row:= &MyRecord{}
//	err := repo.FetchOne(repo.SqlSelect(), row)
func (r *repository) FetchOne(qry *goqu.SelectDataset, target any) error {
	return FetchOne(r.ctx, r.conn, qry, target)
}

// Fetch records; target must be a slice
// Example:
//
//	rows:= make([]*MyRecord,0)
//	err := repo.Fetch(repo.SqlSelect(), rows)
func (r *repository) Fetch(qry *goqu.SelectDataset, target any) error {
	return Fetch(r.ctx, r.conn, qry, target)
}

// FetchRecord fetch a single record with WHERE clause; all clauses are AND
// Example:
//
//	row:= &MyRecord{}
//	err:= repo.FetchRecord(map[string]any{"name":"foo","email":"foo@bar"}, row)
func (r *repository) FetchRecord(fieldValues map[string]any, target any) error {
	return FetchRecord(r.ctx, r.conn, r.SqlSelect(), fieldValues, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (r *repository) FetchByKey(keyField string, value any, target any) error {
	return FetchByKey(r.ctx, r.conn, r.SqlSelect(), keyField, value, target)
}

// FetchWhere fetch multiple records with WHERE clause
func (r *repository) FetchWhere(fieldValues map[string]any, target any) error {
	return FetchWhere(r.ctx, r.conn, r.SqlSelect(), fieldValues, target)
}

// Exec execute a query
func (r *repository) Exec(qry *goqu.SelectDataset) error {
	return Exec(r.ctx, r.conn, qry)
}

// RawExec executes a raw sql query
func (r *repository) RawExec(sql string, args ...any) error {
	return RawExec(r.ctx, r.conn, sql, args...)
}

// Exists returns true if one or more records exist WHERE fieldName=fieldValue
// the optional skip parameter can be used to specify an extra clause WHERE skip[0]<>skip[1]
// Example:
//
//	// check if a record with label == "record 4"
//	exists, err := repo.Exists("label", "record 4")
//	// check if a record with label == "record 4" and id_sample_table<>4 exists
//	exists, err := repo.Exists("label", "record 4", "id_sample_table", 4)
func (r *repository) Exists(fieldName string, fieldValue any, skip ...any) (bool, error) {
	return Exists(r.ctx, r.conn, r.SqlSelect(), fieldName, fieldValue, skip...)
}

func (r *repository) Delete(qry *goqu.DeleteDataset) error {
	return Delete(r.ctx, r.conn, qry)
}

func (r *repository) DeleteWhere(fieldNameValue map[string]any) error {
	return DeleteWhere(r.ctx, r.conn, r.SqlDelete(), fieldNameValue)
}

func (r *repository) DeleteByKey(keyField string, value any) error {
	return DeleteByKey(r.ctx, r.conn, r.SqlDelete(), keyField, value)
}

func (r *repository) Select(sql string, target any, args ...any) error {
	return r.conn.SelectContext(r.ctx, target, sql, args...)
}

func (r *repository) Insert(rows ...any) error {
	qry, args, err := r.sqlBuilder.BuildSQLBatchInsert(r.tableName, rows)
	if err != nil {
		return err
	}
	return RawInsert(r.ctx, r.conn, qry, args)
}

// Count returns the total number of rows in the database table
func (r *repository) Count() (int64, error) {
	return Count(r.ctx, r.conn, r.SqlSelect().Select(goqu.L("COUNT(*)")))
}

// CountWhere returns the number of rows matching the fieldValues map
func (r *repository) CountWhere(fieldValues map[string]any) (int64, error) {
	qry := r.SqlSelect().Select(goqu.L("COUNT(*)"))
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return Count(r.ctx, r.conn, qry)
}

// Grid creates a new Grid object for the record, and caches the field spec for more efficient usage
func (r *repository) Grid(record any) (*Grid, error) {
	return NewGrid(r.tableName, record)
}

// QueryGrid creates a new Grid object and performs a query using GridQuery
func (r *repository) QueryGrid(record any, args *GridQuery, dest any) error {
	var (
		err  error
		qry  *goqu.SelectDataset
		grid *Grid
	)

	grid, err = r.Grid(record)
	if err != nil {
		return err
	}

	qry, err = grid.Build(r.SqlSelect(), args)
	if err != nil {
		return err
	}

	return Fetch(r.ctx, r.conn, qry, dest)
}

// InsertReturning inserts a record, and returns the specified return fields into target
//
// Examples:
//
//	// Struct target (NEW) - automatic field mapping
//	result := &User{}
//	err := repo.InsertReturning(user, []string{"id", "name", "created_at"}, result)
//
//	// Individual variables (EXISTING) - positional mapping
//	var id int64
//	var name string
//	err := repo.InsertReturning(user, []string{"id", "name"}, &id, &name)
//
//	// Single variable (EXISTING)
//	var id int64
//	err := repo.InsertReturning(user, []string{"id"}, &id)
func (r *repository) InsertReturning(record any, returnFields []string, target ...any) error {
	if len(target) == 0 {
		return ErrInvalidParameters
	}

	qry, args, err := r.sqlBuilder.InsertReturning(r.tableName, record, returnFields)
	if err != nil {
		return err
	}

	// Single target: use intelligent scanning (supports structs and single variables)
	if len(target) == 1 {
		return RawInsertReturningFlexible(r.ctx, r.conn, qry, args, target[0])
	}

	// Multiple targets: convert to []any slice and use flexible scanning
	targetSlice := make([]any, len(target))
	copy(targetSlice, target)
	return RawInsertReturningFlexible(r.ctx, r.conn, qry, args, targetSlice)
}

// Update execute an update query
// goqu.InsertDataset/goqu.UpdateDataset() have problems serializing some data types
// Deprecated: use UpdateRecord instead
func (r *repository) Update(qry *goqu.UpdateDataset) error {
	return Update(r.ctx, r.conn, qry)
}

// Do helper to execute a query
func (r *repository) Do(qry any, target ...any) error {
	return Do(r.ctx, r.conn, qry, target...)
}

// UpdateRecord updates a record using a WHERE condition
// whereFieldsValues are matched as field=value, and if multiple entries present, will be
// concatenated using AND
func (r *repository) UpdateRecord(record any, whereFieldsValues map[string]any) error {
	builder := r.sqlBuilder.Update(r.tableName, record).WithOptions(qb.DefaultUpdateOptions())
	if whereFieldsValues != nil && len(whereFieldsValues) > 0 {
		clauses := make([]qb.WhereClause, 0, len(whereFieldsValues))
		for key, value := range whereFieldsValues {
			clauses = append(clauses, qb.Eq(key, value))
		}
		builder = builder.WhereAnd(clauses...)
	}
	qry, args, err := builder.Build()
	if err != nil {
		return err
	}
	return RawExec(r.ctx, r.conn, qry, args...)
}

// UpdateFields update specific fields of a given record, using a WHERE condition
// whereFieldsValues are matched as field=value, and if multiple entries present, will be
// concatenated using AND
func (r *repository) UpdateFields(record any, fieldsValues map[string]any, whereFieldsValues map[string]any) error {
	builder := r.sqlBuilder.Update(r.tableName, record).
		WithOptions(qb.DefaultUpdateOptions()).
		FieldsValues(fieldsValues)
	if whereFieldsValues != nil && len(whereFieldsValues) > 0 {
		clauses := make([]qb.WhereClause, 0, len(whereFieldsValues))
		for key, value := range whereFieldsValues {
			clauses = append(clauses, qb.Eq(key, value))
		}
		builder = builder.WhereAnd(clauses...)
	}
	qry, args, err := builder.Build()
	if err != nil {
		return err
	}
	return RawExec(r.ctx, r.conn, qry, args...)
}

// UpdateReturning updates a record using a WHERE condition with AND
// Examples:
//
//	// Struct target (NEW) - automatic field mapping
//	result := &User{}
//	err := repo.UpdateReturning(user, map[string]any{"id": 1}, []string{"id", "name", "updated_at"}, result)
//
//	// Individual variables (EXISTING) - positional mapping
//	var id int64
//	var name string
//	err := repo.UpdateReturning(user, map[string]any{"id": 1}, []string{"id", "name"}, &id, &name)
//
//	// Single variable (EXISTING)
//	var id int64
//	err := repo.UpdateReturning(user, map[string]any{"id": 1}, []string{"id"}, &id)
func (r *repository) UpdateReturning(record any, whereFieldsValues map[string]any, returnFields []string, target ...any) error {
	if len(target) == 0 {
		return ErrInvalidParameters
	}

	// Set up options with RETURNING fields
	opts := qb.DefaultUpdateOptions()
	opts.ReturningFields = returnFields

	builder := r.sqlBuilder.Update(r.tableName, record).WithOptions(opts)
	if whereFieldsValues != nil && len(whereFieldsValues) > 0 {
		clauses := make([]qb.WhereClause, 0, len(whereFieldsValues))
		for key, value := range whereFieldsValues {
			clauses = append(clauses, qb.Eq(key, value))
		}
		builder = builder.WhereAnd(clauses...)
	}
	qry, args, err := builder.Build()
	if err != nil {
		return err
	}
	return RawUpdateReturning(r.ctx, r.conn, qry, args, target...)
}

// UpdateFieldsReturning updates specific fields using FieldsValues with RETURNING support
// target can be:
// - *struct: for automatic field mapping by name/tag
// - []any: for positional mapping to multiple variables
// - *variable: for single value mapping
func (r *repository) UpdateFieldsReturning(record any, fieldsValues map[string]any, whereFieldsValues map[string]any, returnFields []string, target ...any) error {
	if len(target) == 0 {
		return ErrInvalidParameters
	}

	// Set up options with RETURNING fields
	opts := qb.DefaultUpdateOptions()
	opts.ReturningFields = returnFields

	builder := r.sqlBuilder.Update(r.tableName, record).
		WithOptions(opts).
		FieldsValues(fieldsValues)

	if whereFieldsValues != nil && len(whereFieldsValues) > 0 {
		clauses := make([]qb.WhereClause, 0, len(whereFieldsValues))
		for key, value := range whereFieldsValues {
			clauses = append(clauses, qb.Eq(key, value))
		}
		builder = builder.WhereAnd(clauses...)
	}
	qry, args, err := builder.Build()
	if err != nil {
		return err
	}

	// Single target: use intelligent scanning (supports structs and single variables)
	if len(target) == 1 {
		return RawUpdateReturningFlexible(r.ctx, r.conn, qry, args, target[0])
	}

	// Multiple targets: convert to []any slice and use flexible scanning
	targetSlice := make([]any, len(target))
	copy(targetSlice, target)
	return RawUpdateReturningFlexible(r.ctx, r.conn, qry, args, targetSlice)
}

// UpdateByKey updates a record using WHERE keyField=value
func (r *repository) UpdateByKey(record any, keyField string, value any) error {
	qry, args, err := r.sqlBuilder.Update(r.tableName, record).
		WithOptions(qb.DefaultUpdateOptions()).
		Where(qb.Eq(keyField, value)).
		Build()
	if err != nil {
		return err
	}
	return RawExec(r.ctx, r.conn, qry, args...)
}

func (t *tx) SqlDialect() qb.SqlDialect {
	return t.sqlBuilder.Dialect()
}

func (t *tx) SqlBuilder() *qb.SqlBuilder {
	return t.sqlBuilder
}

func (t *tx) SqlUpdateX(record any) *qb.UpdateBuilder {
	return t.SqlBuilder().Update(t.tableName, record)
}

func (t *tx) Commit() error {
	return t.conn.Commit()
}

func (t *tx) Rollback() error {
	return t.conn.Rollback()
}

func (t *tx) Db() *sqlx.Tx {
	return t.conn
}

func (t *tx) Name() string {
	return t.tableName
}

func (t *tx) Sql() goqu.DialectWrapper {
	return t.dialect
}

func (t *tx) SqlSelect() *goqu.SelectDataset {
	return t.dialect.From(t.tableName)
}

// SqlInsert
// Compatibility note: goqu prepared statements are *not* compatible with postgresql extended
// types; build update clauses with SqlBuilder() instead
func (t *tx) SqlInsert() *goqu.InsertDataset {
	return t.dialect.Insert(t.tableName)
}

// SqlUpdate
// Compatibility note: goqu prepared statements are *not* compatible with postgresql extended
// types; build update clauses with SqlBuilder() instead
func (t *tx) SqlUpdate() *goqu.UpdateDataset {
	return t.dialect.Update(t.tableName)
}

func (t *tx) SqlDelete() *goqu.DeleteDataset {
	return t.dialect.Delete(t.tableName)
}

// FetchOne fetch a record; target must be a struct
func (t *tx) FetchOne(qry *goqu.SelectDataset, target any) error {
	return FetchOne(t.ctx, t.conn, qry, target)
}

func (t *tx) Fetch(qry *goqu.SelectDataset, target any) error {
	return Fetch(t.ctx, t.conn, qry, target)
}

// FetchRecord fetch a single record with WHERE clause
func (t *tx) FetchRecord(fieldValues map[string]any, target any) error {
	return FetchRecord(t.ctx, t.conn, t.SqlSelect(), fieldValues, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (t *tx) FetchByKey(keyField string, value any, target any) error {
	return FetchByKey(t.ctx, t.conn, t.SqlSelect(), keyField, value, target)
}

// FetchWhere fetch multiple records with WHERE clause
func (t *tx) FetchWhere(fieldValues map[string]any, target any) error {
	return FetchWhere(t.ctx, t.conn, t.SqlSelect(), fieldValues, target)
}

// Exists returns true if one or more records exist WHERE fieldName=fieldValue
// the optional skip parameter can be used to specify an extra clause WHERE skip[0]<>skip[1]
func (t *tx) Exists(fieldName string, fieldValue any, skip ...any) (bool, error) {
	return Exists(t.ctx, t.conn, t.SqlSelect(), fieldName, fieldValue, skip...)
}

func (t *tx) Exec(qry *goqu.SelectDataset) error {
	return Exec(t.ctx, t.conn, qry)
}

func (t *tx) RawExec(sql string, args ...any) error {
	return RawExec(t.ctx, t.conn, sql, args...)
}

func (t *tx) Delete(qry *goqu.DeleteDataset) error {
	return Delete(t.ctx, t.conn, qry)
}

func (t *tx) DeleteWhere(fieldNameValue map[string]any) error {
	return DeleteWhere(t.ctx, t.conn, t.SqlDelete(), fieldNameValue)
}

func (t *tx) DeleteByKey(keyField string, value any) error {
	return DeleteByKey(t.ctx, t.conn, t.SqlDelete(), keyField, value)
}

func (t *tx) Select(sql string, target any, args ...any) error {
	return t.conn.SelectContext(t.ctx, target, sql, args...)
}

func (t *tx) Insert(rows ...any) error {
	qry, args, err := t.sqlBuilder.BuildSQLBatchInsert(t.tableName, rows)
	if err != nil {
		return err
	}
	return RawInsert(t.ctx, t.conn, qry, args)
}

// InsertReturning inserts a record, and returns the specified return fields into target
func (t *tx) InsertReturning(record any, returnFields []string, target ...any) error {
	qry, args, err := t.sqlBuilder.InsertReturning(t.tableName, record, returnFields)
	if err != nil {
		return err
	}

	if len(target) == 0 {
		return ErrInvalidParameters
	}

	// Single target: use intelligent scanning (supports structs and single variables)
	if len(target) == 1 {
		return RawInsertReturningFlexible(t.ctx, t.conn, qry, args, target[0])
	}

	// Multiple targets: convert to []any slice and use flexible scanning
	targetSlice := make([]any, len(target))
	copy(targetSlice, target)
	return RawInsertReturningFlexible(t.ctx, t.conn, qry, args, targetSlice)
}

// Update execute an update query
func (t *tx) Update(qry *goqu.UpdateDataset) error {
	return Update(t.ctx, t.conn, qry)
}

// Do helper to execute a query
func (t *tx) Do(qry any, target ...any) error {
	return Do(t.ctx, t.conn, qry, target)
}

// UpdateRecord updates a record using a WHERE condition
// whereFieldsValues are matched as field=value, and if multiple entries present, will be
// concatenated using AND
func (t *tx) UpdateRecord(record any, whereFieldsValues map[string]any) error {
	builder := t.sqlBuilder.Update(t.tableName, record).WithOptions(qb.DefaultUpdateOptions())
	if whereFieldsValues != nil && len(whereFieldsValues) > 0 {
		clauses := make([]qb.WhereClause, 0, len(whereFieldsValues))
		for key, value := range whereFieldsValues {
			clauses = append(clauses, qb.Eq(key, value))
		}
		builder = builder.WhereAnd(clauses...)
	}
	qry, args, err := builder.Build()
	if err != nil {
		return err
	}
	return RawExec(t.ctx, t.conn, qry, args...)
}

// UpdateFields update specific fields of a given record, using a WHERE condition
// whereFieldsValues are matched as field=value, and if multiple entries present, will be
// concatenated using AND
func (t *tx) UpdateFields(record any, fieldsValues map[string]any, whereFieldsValues map[string]any) error {
	builder := t.sqlBuilder.Update(t.tableName, record).
		WithOptions(qb.DefaultUpdateOptions()).
		FieldsValues(fieldsValues)
	if whereFieldsValues != nil && len(whereFieldsValues) > 0 {
		clauses := make([]qb.WhereClause, 0, len(whereFieldsValues))
		for key, value := range whereFieldsValues {
			clauses = append(clauses, qb.Eq(key, value))
		}
		builder = builder.WhereAnd(clauses...)
	}
	qry, args, err := builder.Build()
	if err != nil {
		return err
	}
	return RawExec(t.ctx, t.conn, qry, args...)
}

// UpdateReturning updates a record using a WHERE condition with AND
// Examples:
//
//	// Struct target (NEW) - automatic field mapping
//	result := &User{}
//	err := repo.UpdateReturning(user, map[string]any{"id": 1}, []string{"id", "name", "updated_at"}, result)
//
//	// Individual variables (EXISTING) - positional mapping
//	var id int64
//	var name string
//	err := repo.UpdateReturning(user, map[string]any{"id": 1}, []string{"id", "name"}, &id, &name)
//
//	// Single variable (EXISTING)
//	var id int64
//	err := repo.UpdateReturning(user, map[string]any{"id": 1}, []string{"id"}, &id)
func (t *tx) UpdateReturning(record any, whereFieldsValues map[string]any, returnFields []string, target ...any) error {
	if len(target) == 0 {
		return ErrInvalidParameters
	}

	// Set up options with RETURNING fields
	opts := qb.DefaultUpdateOptions()
	opts.ReturningFields = returnFields

	builder := t.sqlBuilder.Update(t.tableName, record).WithOptions(opts)
	if whereFieldsValues != nil && len(whereFieldsValues) > 0 {
		clauses := make([]qb.WhereClause, 0, len(whereFieldsValues))
		for key, value := range whereFieldsValues {
			clauses = append(clauses, qb.Eq(key, value))
		}
		builder = builder.WhereAnd(clauses...)
	}
	qry, args, err := builder.Build()
	if err != nil {
		return err
	}

	// Single target: use intelligent scanning (supports structs and single variables)
	if len(target) == 1 {
		return RawUpdateReturningFlexible(t.ctx, t.conn, qry, args, target[0])
	}

	// Multiple targets: convert to []any slice and use flexible scanning
	targetSlice := make([]any, len(target))
	copy(targetSlice, target)
	return RawUpdateReturningFlexible(t.ctx, t.conn, qry, args, targetSlice)
}

// UpdateFieldsReturning updates specific fields using FieldsValues with RETURNING support
// target can be:
// - *struct: for automatic field mapping by name/tag
// - []any: for positional mapping to multiple variables
// - *variable: for single value mapping
func (t *tx) UpdateFieldsReturning(record any, fieldsValues map[string]any, whereFieldsValues map[string]any, returnFields []string, target ...any) error {
	if len(target) == 0 {
		return ErrInvalidParameters
	}

	// Set up options with RETURNING fields
	opts := qb.DefaultUpdateOptions()
	opts.ReturningFields = returnFields

	builder := t.sqlBuilder.Update(t.tableName, record).
		WithOptions(opts).
		FieldsValues(fieldsValues)

	if whereFieldsValues != nil && len(whereFieldsValues) > 0 {
		clauses := make([]qb.WhereClause, 0, len(whereFieldsValues))
		for key, value := range whereFieldsValues {
			clauses = append(clauses, qb.Eq(key, value))
		}
		builder = builder.WhereAnd(clauses...)
	}
	qry, args, err := builder.Build()
	if err != nil {
		return err
	}

	// Single target: use intelligent scanning (supports structs and single variables)
	if len(target) == 1 {
		return RawUpdateReturningFlexible(t.ctx, t.conn, qry, args, target[0])
	}

	// Multiple targets: convert to []any slice and use flexible scanning
	targetSlice := make([]any, len(target))
	copy(targetSlice, target)
	return RawUpdateReturningFlexible(t.ctx, t.conn, qry, args, targetSlice)
}

// UpdateByKey updates a record using WHERE keyField=value
func (t *tx) UpdateByKey(record any, keyField string, value any) error {
	qry, args, err := t.sqlBuilder.Update(t.tableName, record).
		WithOptions(qb.DefaultUpdateOptions()).
		Where(qb.Eq(keyField, value)).
		Build()
	if err != nil {
		return err
	}
	return RawExec(t.ctx, t.conn, qry, args...)
}

// Count returns the total number of rows in the database table
func (t *tx) Count() (int64, error) {
	return Count(t.ctx, t.conn, t.SqlSelect().Select(goqu.L("COUNT(*)")))
}

// CountWhere returns the number of rows matching the fieldValues map
func (t *tx) CountWhere(fieldValues map[string]any) (int64, error) {
	qry := t.SqlSelect().Select(goqu.L("COUNT(*)"))
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return Count(t.ctx, t.conn, qry)
}

// EmptyResult returns true if error is empty result
func EmptyResult(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}
