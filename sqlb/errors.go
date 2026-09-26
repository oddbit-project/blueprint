package sqlb

import (
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrUnknownDialect    = utils.Error("sqlb: unknown or zero dialect")
	ErrInvalidIdentifier = utils.Error("sqlb: invalid identifier")
	ErrUnsupported       = utils.Error("sqlb: not supported by dialect")
	ErrRawArgs           = utils.Error("sqlb: raw placeholder count does not match arguments")
	ErrRawPlaceholder    = utils.Error("sqlb: raw sql contains a forbidden placeholder sequence")
	ErrInvalidFunction   = utils.Error("sqlb: invalid function name")
	ErrInvalidType       = utils.Error("sqlb: invalid cast type")
	ErrEmptyMatch        = utils.Error("sqlb: Match requires at least one field")
	ErrEmptyCase         = utils.Error("sqlb: CASE requires at least one WHEN")
	ErrUnsafeValue       = utils.Error("sqlb: value type cannot be bound safely for this dialect")
	ErrTooManyArgs       = utils.Error("sqlb: statement exceeds the dialect's bound-argument limit")
	ErrNilExpr           = utils.Error("sqlb: nil or zero expression")
	ErrNoWhere           = utils.Error("sqlb: statement requires a WHERE clause; call All() to affect every row")
	ErrNoTable           = utils.Error("sqlb: statement has no table")
	ErrNeedAlias         = utils.Error("sqlb: subquery in FROM requires an alias")
	ErrInvalidColumn     = utils.Error("sqlb: column must be a string or an expression")
	ErrInvalidLimit      = utils.Error("sqlb: LIMIT/OFFSET out of range for dialect")
)
