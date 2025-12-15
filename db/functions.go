package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db/field"
	"github.com/oddbit-project/blueprint/db/qb"
)

type SqlAdapter interface {
	sqlx.QueryerContext
	sqlx.ExecerContext
	SqlxReaderCtx
}

func RawExec(ctx context.Context, conn sqlx.ExecerContext, sql string, args ...any) error {
	_, err := conn.ExecContext(ctx, sql, args...)
	return err
}

func RawFetch(ctx context.Context, conn sqlx.QueryerContext, sql string, args []any, target any) error {
	if target == nil {
		return ErrInvalidParameters
	}
	return conn.QueryRowxContext(ctx, sql, args...).StructScan(target)
}

func Exec(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.SelectDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return RawExec(ctx, conn, sqlQry, args...)
}

func FetchOne(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, target any) error {
	if target == nil || qry == nil {
		return ErrInvalidParameters
	}
	qry = qry.Limit(1)
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return conn.QueryRowxContext(ctx, sqlQry, args...).StructScan(target)
}

func Fetch(ctx context.Context, conn SqlxReaderCtx, qry *goqu.SelectDataset, target any) error {
	if target == nil || qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return conn.SelectContext(ctx, target, sqlQry, args...)
}

func FetchRecord(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return FetchOne(ctx, conn, qry, target)
}

func FetchByKey(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, keyField string, value any, target any) error {
	if target == nil {
		return ErrInvalidParameters
	}
	return FetchOne(ctx, conn, qry.Where(goqu.C(keyField).Eq(value)), target)
}

func FetchWhere(ctx context.Context, conn SqlxReaderCtx, qry *goqu.SelectDataset, fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return Fetch(ctx, conn, qry, target)
}

func Exists(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, fieldName string, fieldValue any, skip ...any) (bool, error) {
	result := 0
	qry = qry.Select(goqu.L("COUNT(*)")).Where(goqu.C(fieldName).Eq(fieldValue))
	if len(skip) > 0 {
		if len(skip) != 2 {
			return false, ErrInvalidParameters
		}
		qry = qry.Where(goqu.C(skip[0].(string)).Neq(skip[1]))
	}
	qrySql, args, err := qry.ToSQL()
	if err != nil {
		return false, err
	}

	if err = conn.QueryRowxContext(ctx, qrySql, args...).Scan(&result); err != nil {
		return false, err
	}
	return result > 0, err
}

func Delete(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, sqlQry, args...)
	return err
}

func DeleteWhere(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset, fieldNameValue map[string]any) error {
	if fieldNameValue == nil {
		return ErrInvalidParameters
	}
	for field, value := range fieldNameValue {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return Delete(ctx, conn, qry)
}

func DeleteByKey(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset, keyField string, value any) error {
	qry = qry.Where(goqu.C(keyField).Eq(value))
	return Delete(ctx, conn, qry)
}

func RawInsert(ctx context.Context, conn sqlx.ExecerContext, qry string, values []any) error {
	if len(values) == 0 {
		return ErrInvalidParameters
	}
	_, err := conn.ExecContext(ctx, qry, values...)
	return err
}

func RawInsertReturning(ctx context.Context, conn sqlx.QueryerContext, qry string, values []any, target ...any) error {
	if len(target) == 0 {
		return ErrInvalidParameters
	}
	return conn.QueryRowxContext(ctx, qry, values...).Scan(target...)
}

// RawInsertReturningFlexible provides intelligent target scanning for InsertReturning operations.
// It automatically detects the target type and uses the appropriate scanning method:
// - *struct: uses StructScan() for automatic field mapping by name/tag
// - []any: uses Scan() for positional mapping to multiple variables
// - *variable: uses Scan() for single value mapping
func RawInsertReturningFlexible(ctx context.Context, conn sqlx.QueryerContext, sql string, args []any, target any) error {
	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	// Execute the query
	row := conn.QueryRowxContext(ctx, sql, args...)

	// Intelligent type detection and scanning
	return scanTarget(row, target)
}

// scanTarget determines the appropriate scanning method based on target type
func scanTarget(row *sqlx.Row, target any) error {
	targetValue := reflect.ValueOf(target)

	if !targetValue.IsValid() {
		return fmt.Errorf("invalid target value")
	}

	targetType := targetValue.Type()

	switch {
	case targetType.Kind() == reflect.Ptr && targetType.Elem().Kind() == reflect.Struct:
		// reserved structs are parsed as a single field, eg. time.Time
		if field.IsReservedType(strings.Replace(targetType.String(), "*", "", 1)) {
			return row.Scan(target)
		}
		// Struct pointer - use StructScan for field mapping by name/tag
		return row.StructScan(target)

	case targetType.Kind() == reflect.Slice:
		// Slice of variables - convert to []any and use positional Scan
		slice, ok := target.([]any)
		if !ok {
			return fmt.Errorf("slice target must be []any, got %T", target)
		}
		return row.Scan(slice...)

	case targetType.Kind() == reflect.Ptr:
		// Single variable pointer - use direct Scan
		return row.Scan(target)

	default:
		return fmt.Errorf("unsupported target type %T: expected *struct, []any, or *variable", target)
	}
}

// RawUpdateReturning
// helper function for RawUpdateReturningFlexible
func RawUpdateReturning(ctx context.Context, conn sqlx.QueryerContext, qry string, args []any, target ...any) error {
	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	// Single target: use intelligent scanning (supports structs and single variables)
	if len(target) == 1 {
		return RawUpdateReturningFlexible(ctx, conn, qry, args, target[0])
	}

	// Multiple targets: convert to []any slice and use flexible scanning
	targetSlice := make([]any, len(target))
	copy(targetSlice, target)
	return RawUpdateReturningFlexible(ctx, conn, qry, args, targetSlice)
}

// RawUpdateReturningFlexible provides intelligent target scanning for UpdateReturning operations.
// It automatically detects the target type and uses the appropriate scanning method:
// - *struct: uses StructScan() for automatic field mapping by name/tag
// - []any: uses Scan() for positional mapping to multiple variables
// - *variable: uses Scan() for single value mapping
func RawUpdateReturningFlexible(ctx context.Context, conn sqlx.QueryerContext, sql string, args []any, target any) error {
	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	// Execute the query
	row := conn.QueryRowxContext(ctx, sql, args...)

	// Use the same intelligent type detection and scanning as InsertReturning
	return scanTarget(row, target)
}

func Do(ctx context.Context, conn SqlAdapter, qry any, target ...any) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	switch qry.(type) {
	case *goqu.SelectDataset:
		if target == nil {
			return ErrInvalidParameters
		}
		return Fetch(ctx, conn, qry.(*goqu.SelectDataset), target[0])
	case *goqu.UpdateDataset:
		return Update(ctx, conn, qry.(*goqu.UpdateDataset))
	case *goqu.InsertDataset:
		gQry := qry.(*goqu.InsertDataset)
		sqlQry, args, err := gQry.Prepared(true).ToSQL()
		if err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, sqlQry, args...)
		return err

	case *goqu.DeleteDataset:
		return Delete(ctx, conn, qry.(*goqu.DeleteDataset))

	case *qb.UpdateBuilder:
		param := qry.(*qb.UpdateBuilder)
		qrySql, args, err := param.Build()
		if err != nil {
			return err
		}
		if param.HasReturnFields() {
			if target == nil {
				return ErrInvalidParameters
			}
			return RawUpdateReturning(ctx, conn, qrySql, args, target...)
		} else {
			// no return fields
			return RawExec(ctx, conn, qrySql, args...)
		}
	default:
		return ErrInvalidParameters
	}
}

func Update(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.UpdateDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	qrySql, values, err := qry.Prepared(true).ToSQL()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, qrySql, values...)
	return err
}

func Count(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset) (int64, error) {
	sqlQry, values, err := qry.ToSQL()
	if err != nil {
		return 0, err
	}
	var count int64
	if err = conn.QueryRowxContext(ctx, sqlQry, values...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
