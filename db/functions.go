package db

import (
	"context"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

func RawExec(ctx context.Context, conn sqlx.ExecerContext, sql string, args ...any) error {
	_, err := conn.ExecContext(ctx, sql, args...)
	return err
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
	qry.Limit(1)
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

func Del(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset) error {
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
	return Del(ctx, conn, qry)
}

func DeleteByKey(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset, keyField string, value any) error {
	qry = qry.Where(goqu.C(keyField).Eq(value))
	return Del(ctx, conn, qry)
}

func Insert(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.InsertDataset, rows ...any) error {
	if len(rows) == 0 {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.Rows(rows...).Prepared(true).ToSQL()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, sqlQry, args...)
	return err
}

func InsertReturning(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.InsertDataset, record any, returnFields []interface{}, target ...any) error {
	if record == nil || returnFields == nil || len(target) == 0 {
		return ErrInvalidParameters
	}
	sqlQry, values, err := qry.Rows(record).Prepared(true).Returning(returnFields...).ToSQL()
	if err != nil {
		return err
	}
	return conn.QueryRowxContext(ctx, sqlQry, values...).Scan(target...)
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

func UpdateRecord(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.UpdateDataset, record any, whereFieldsValues map[string]any) error {
	if record == nil {
		return ErrInvalidParameters
	}
	qry = qry.Set(record)
	if whereFieldsValues != nil {
		for field, value := range whereFieldsValues {
			qry = qry.Where(goqu.C(field).Eq(value))
		}
	}
	return Update(ctx, conn, qry)
}

func UpdateByKey(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.UpdateDataset, record any, keyField string, value any) error {
	if record == nil {
		return ErrInvalidParameters
	}
	return Update(ctx, conn, qry.Set(record).Where(goqu.C(keyField).Eq(value)))
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
