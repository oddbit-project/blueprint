module github.com/oddbit-project/blueprint/provider/pgsql

go 1.23.0

require (
	github.com/oddbit-project/blueprint v0.8.0
	github.com/doug-martin/goqu/v9 v9.19.0
	github.com/jackc/pgx/v5 v5.7.5
	github.com/jmoiron/sqlx v1.4.0
	github.com/lib/pq v1.10.9
	github.com/testcontainers/testcontainers-go v0.38.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.38.0
)

replace github.com/oddbit-project/blueprint => ../../