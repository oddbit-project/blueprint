# blueprint.provider.clickhouse

Blueprint ClickHouse client

## Using the client

The ClickHouse client relies on a single DSN string:

```json
{
  "clickhouse": {
    "dsn": "clickhouse://username:password@host1:9000/database?dial_timeout=200ms&max_execution_time=600&secure=true"
  }
}
```


```go
package main

import (
	"fmt"
	"github.com/oddbit-project/blueprint/provider/clickhouse"
	"log"
	"os"
)

func main() {
	chConfig := &clickhouse.ClientConfig{
		DSN: "clickhouse://default:password@localhost:9000/default",
	}

	client, err := clickhouse.NewClient(chConfig)
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