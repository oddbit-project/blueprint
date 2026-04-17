# sqlite sample

Minimal end-to-end example of the `provider/sqlite` provider:

- opens a SQLite database file (pure-Go driver, no CGO)
- applies an embedded migration
- inserts and reads records through the repository

## Run

```
go run ./samples/sqlite
```

The database file is created at `$TMPDIR/blueprint-sqlite-sample.db` and wiped on each run.
