package qb

type SqlBuilder struct {
	dialect SqlDialect
}

func NewSqlBuilder(dialect SqlDialect) *SqlBuilder {
	return &SqlBuilder{
		dialect: dialect,
	}
}

func (s *SqlBuilder) Dialect() SqlDialect {
	return s.dialect
}
