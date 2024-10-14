# blueprint.provider.pgsql

Blueprint PostgreSQL client

The client uses the [pgx](https://github.com/jackc/pgx) library.


## Using the client

The PostgreSQL client relies on a single DSN string:

```json
{
  "clickhouse": {
    "dsn": "postgres://username:password@localhost:5432/database?sslmode=allow"
  }
}
```


```go
package main

import (
	"fmt"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
	"os"
)

func main() {
	pgConfig := pgsql.NewClientConfig()
	pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"

	client, err := pgsql.NewClient(pgConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect()
	
	// do stuff
}

```