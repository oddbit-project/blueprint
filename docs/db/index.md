# Database

Blueprint's database layer has two APIs over the same connection:

- **[`dbx`](dbx.md) and [`gohan`](gohan.md)**: typed generic repositories (`dbx.Repository[T]`)
  built on the `gohan` query builder. Values are always bound and identifiers always escaped, and
  every call takes a `context.Context`. **Use these for new code.**
- **The legacy [`db` package](legacy-overview.md)**: `db.Repository` and `db.Grid`, built on goqu.
  It is kept for existing code and receives security fixes only. See
  [Migrating from db to dbx](migrating-to-dbx.md) to move a repository at a time.

Both work over the same `*db.SqlClient`, so an application can migrate incrementally.

## Quick start

A `dbx` repository over SQLite (no server needed):

```go
package main

import (
    "context"
    "fmt"

    "github.com/oddbit-project/blueprint/dbx"
    "github.com/oddbit-project/blueprint/provider/sqlite"
    "github.com/oddbit-project/gohan"
)

type User struct {
    ID    int64  `db:"id,auto" json:"id"`
    Name  string `db:"name" json:"name"`
    Email string `db:"email" json:"email"`
}

func main() {
    ctx := context.Background()

    cfg := sqlite.NewClientConfig()
    cfg.DSN = "file:app.db"
    client, err := sqlite.NewClient(cfg)
    if err != nil {
        panic(err)
    }
    defer client.Disconnect()

    if _, err := client.Db().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE)`); err != nil {
        panic(err)
    }

    q, err := dbx.FromClient(client)
    if err != nil {
        panic(err)
    }
    users, err := dbx.NewRepository[User](q, "users")
    if err != nil {
        panic(err)
    }

    if _, err := users.InsertIgnore(ctx, &User{Name: "Alice", Email: "alice@example.com"}, "email"); err != nil {
        panic(err)
    }
    alice, err := users.Get(ctx, users.Select().Where(gohan.Col("email").Eq("alice@example.com")))
    if err != nil {
        panic(err)
    }
    fmt.Println(alice.ID, alice.Name)
}
```

Use schema migrations rather than `CREATE TABLE` in application code; see [Migrations](migrations.md).

## Where to go next

| Topic | Page |
|---|---|
| Repositories: reads, writes, upserts, transactions, retries, grids, cursor pagination, PATCH | [dbx Repositories](dbx.md) |
| Building queries: expressions, joins, subqueries, dialects | [gohan Query Builder](gohan.md) |
| Struct tags (`db`, `ch`, `auto`, `omitempty`, `grid`) | [Structs and Tags](structs-and-tags.md) |
| Connections and pooling | [Database Client](client.md) |
| Schema migrations | [Migrations](migrations.md) |
| Driver setup | [PostgreSQL](../provider/pgsql.md), [SQLite](../provider/sqlite.md), [ClickHouse](../provider/clickhouse.md) |
| Moving from `db` to `dbx` | [Migrating from db to dbx](migrating-to-dbx.md) |

## Drivers

| Driver | Connect | `dbx` Querier |
|---|---|---|
| PostgreSQL | `pgsql.NewClient(cfg)` → `*db.SqlClient` | `dbx.FromClient(client)` |
| SQLite | `sqlite.NewClient(cfg)` → `*db.SqlClient` | `dbx.FromClient(client)` |
| ClickHouse | `clickhouse.NewClient(cfg)` → `*clickhouse.Client` | `client.Querier()` |

Not every feature is available on every database (ClickHouse has no transactions, `RETURNING` or
upserts); see the dialect support table in [dbx Repositories](dbx.md).
