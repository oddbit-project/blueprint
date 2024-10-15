# db.Repository

Repository pattern implementation with blueprint and goqu

## Usage
 
```go
package main

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
	"time"
)

type UserRecord struct {
	Id        int       `db:"id_user" goqu:"skipinsert"` // field is autogenerated
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
}

func main() {
	pgConfig := pgsql.NewClientConfig() // use defaults
	pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"

	client, err := pgsql.NewClient(pgConfig)
	if err != nil {
		log.Fatal(err)
	}

	// create a repository for the table users
	// Note: context is internally stored and then propagated to the appropriate sqlx methods; this is
	// not the advised way of using contexts, but the rationale is to allow clean thread or application shutdown
	// via context, without the overhead of adding an extra parameter to every function
	repo := db.NewRepository(context.Background(), client, "users")

	user1 := &UserRecord{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      "John Connor",
		Email:     "jconnor@skynet.com",
	}

	// Add user
	if err = repo.Insert(user1); err != nil {
		log.Fatal(err)
	}

	// Read all users
	users := make([]*UserRecord, 0)
	if err = repo.Fetch(repo.SqlSelect(), &users); err != nil {
		log.Fatal(err)
	}
	
	// search for sarah by email
	sarah := &UserRecord{}
	if err = repo.FetchRecord(map[string]any{"email": "sconnor@skynet.com"}, sarah); err != nil {
		if db.EmptyResult(err) {
			fmt.Println("Sarah Connor not found")
		} else {
			log.Fatal(err)
		}
			
	}
}
```