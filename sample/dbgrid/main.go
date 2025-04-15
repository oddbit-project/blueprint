package main

import (
	"context"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/provider/pgsql"
	"log"
	"os"
	"strconv"
	"time"
)

// User represents a user record in the database
type User struct {
	ID        int       `db:"id" json:"id" grid:"sort,filter"`
	Username  string    `db:"username" json:"username" grid:"sort,search,filter"`
	Email     string    `db:"email" json:"email" grid:"sort,search,filter"`
	Active    bool      `db:"active" json:"active" grid:"filter"`
	Role      string    `db:"role" json:"role" grid:"sort,filter"`
	CreatedAt time.Time `db:"created_at" json:"createdAt" grid:"sort"`
}

func main() {
	// Check if this is a demo mode or should try to connect to a real DB
	realDB := false
	if len(os.Args) > 1 && os.Args[1] == "--connect" {
		realDB = true
	}

	fmt.Println("DB Grid Sample")
	fmt.Println("==============")

	// Initialize grid with the User struct
	userGrid, err := db.NewGrid("users", &User{})
	if err != nil {
		log.Fatal("Error creating grid: ", err)
	}

	// Add filter functions for specific fields
	userGrid.AddFilterFunc("active", func(value any) (any, error) {
		// Convert various string formats to boolean
		switch v := value.(type) {
		case string:
			switch v {
			case "1", "true", "yes", "y", "on":
				return true, nil
			case "0", "false", "no", "n", "off":
				return false, nil
			default:
				return nil, db.GridError{
					Scope:   "filter",
					Field:   "active",
					Message: "invalid boolean value",
				}
			}
		case bool:
			return v, nil
		case int:
			return v != 0, nil
		default:
			return nil, db.GridError{
				Scope:   "filter",
				Field:   "active",
				Message: "type not supported",
			}
		}
	})

	userGrid.AddFilterFunc("id", func(value any) (any, error) {
		// Convert string ID to integer
		switch v := value.(type) {
		case string:
			id, err := strconv.Atoi(v)
			if err != nil {
				return nil, db.GridError{
					Scope:   "filter",
					Field:   "id",
					Message: "invalid numeric value",
				}
			}
			return id, nil
		case int:
			return v, nil
		default:
			return nil, db.GridError{
				Scope:   "filter",
				Field:   "id",
				Message: "type not supported",
			}
		}
	})

	// ==== Example 1: Simple Query ====
	fmt.Println("\nExample 1: Simple Query")
	fmt.Println("- Limit: 10 records")
	fmt.Println("- Offset: 0")
	fmt.Println("- Sorting: username ascending")

	query1, err := db.NewGridQuery(db.SearchNone, 10, 0)
	if err != nil {
		log.Fatal("Error creating grid query: ", err)
	}
	
	// Add sorting by username
	query1.SortFields = map[string]string{
		"username": db.SortAscending,
	}

	// Validate the query
	if err := userGrid.ValidQuery(query1); err != nil {
		log.Fatal("Invalid query: ", err)
	}
	
	// Build the SQL
	statement1, err := userGrid.Build(nil, query1)
	if err != nil {
		log.Fatal("Error building SQL: ", err)
	}
	
	// Convert to SQL
	sql1, args1, err := statement1.ToSQL()
	if err != nil {
		log.Fatal("Error generating SQL: ", err)
	}
	
	fmt.Println("SQL:", sql1)
	fmt.Println("Args:", args1)

	// ==== Example 2: Filtering ====
	fmt.Println("\nExample 2: Filtering")
	fmt.Println("- Filter: active=yes, role=admin")
	fmt.Println("- Limit: 5 records")
	
	query2, err := db.NewGridQuery(db.SearchNone, 5, 0)
	if err != nil {
		log.Fatal("Error creating grid query: ", err)
	}
	
	// Add filters
	query2.FilterFields = map[string]any{
		"active": "yes",
		"role":   "admin",
	}

	// Validate the query
	if err := userGrid.ValidQuery(query2); err != nil {
		log.Fatal("Invalid query: ", err)
	}
	
	// Build the SQL
	statement2, err := userGrid.Build(nil, query2)
	if err != nil {
		log.Fatal("Error building SQL: ", err)
	}
	
	// Convert to SQL
	sql2, args2, err := statement2.ToSQL()
	if err != nil {
		log.Fatal("Error generating SQL: ", err)
	}
	
	fmt.Println("SQL:", sql2)
	fmt.Println("Args:", args2)

	// ==== Example 3: Searching ====
	fmt.Println("\nExample 3: Searching")
	fmt.Println("- Search: 'john' in searchable fields")
	fmt.Println("- Search Type: Any (contains)")
	
	query3, err := db.NewGridQuery(db.SearchAny, 10, 0)
	if err != nil {
		log.Fatal("Error creating grid query: ", err)
	}
	
	query3.SearchText = "john"

	// Validate the query
	if err := userGrid.ValidQuery(query3); err != nil {
		log.Fatal("Invalid query: ", err)
	}
	
	// Build the SQL
	statement3, err := userGrid.Build(nil, query3)
	if err != nil {
		log.Fatal("Error building SQL: ", err)
	}
	
	// Convert to SQL
	sql3, args3, err := statement3.ToSQL()
	if err != nil {
		log.Fatal("Error generating SQL: ", err)
	}
	
	fmt.Println("SQL:", sql3)
	fmt.Println("Args:", args3)

	// ==== Example 4: Complex Query ====
	fmt.Println("\nExample 4: Complex Query")
	fmt.Println("- Search: 'smith' in searchable fields")
	fmt.Println("- Filter: active=true")
	fmt.Println("- Sort: id descending")
	fmt.Println("- Limit: 15, Offset: 30")
	
	query4, err := db.NewGridQuery(db.SearchAny, 15, 30)
	if err != nil {
		log.Fatal("Error creating grid query: ", err)
	}
	
	query4.SearchText = "smith"
	query4.FilterFields = map[string]any{
		"active": true,
	}
	query4.SortFields = map[string]string{
		"id": db.SortDescending,
	}

	// Validate the query
	if err := userGrid.ValidQuery(query4); err != nil {
		log.Fatal("Invalid query: ", err)
	}
	
	// Build the SQL
	statement4, err := userGrid.Build(nil, query4)
	if err != nil {
		log.Fatal("Error building SQL: ", err)
	}
	
	// Convert to SQL
	sql4, args4, err := statement4.ToSQL()
	if err != nil {
		log.Fatal("Error generating SQL: ", err)
	}
	
	fmt.Println("SQL:", sql4)
	fmt.Println("Args:", args4)

	// Try connecting to a real database if requested
	if realDB {
		fmt.Println("\nConnecting to database...")
		
		// Connect to PostgreSQL
		pgConfig := pgsql.NewClientConfig()
		pgConfig.DSN = "postgres://username:password@localhost:5432/database?sslmode=allow"
		
		client, err := pgsql.NewClient(pgConfig)
		if err != nil {
			log.Fatal("Error connecting to database: ", err)
		}
		defer client.Disconnect()
		
		// Execute a query using the database connection
		sqlStr, args, err := statement1.ToSQL()
		if err != nil {
			log.Fatal("Error generating SQL: ", err)
		}
		
		rows, err := client.Db().QueryxContext(context.Background(), sqlStr, args...)
		if err != nil {
			log.Fatal("Error executing query: ", err)
		}
		defer rows.Close()
		
		// Display results
		fmt.Println("\nQuery Results:")
		fmt.Println("-------------")
		
		var users []User
		for rows.Next() {
			var user User
			if err := rows.StructScan(&user); err != nil {
				log.Fatal("Error scanning row: ", err)
			}
			users = append(users, user)
		}
		
		if err := rows.Err(); err != nil {
			log.Fatal("Error iterating rows: ", err)
		}
		
		// Display users
		for _, user := range users {
			fmt.Printf("ID: %d, Username: %s, Email: %s, Active: %t, Role: %s, Created: %s\n",
				user.ID, user.Username, user.Email, user.Active, user.Role, user.CreatedAt.Format(time.RFC3339))
		}
	}

	// ==== Example 5: Custom Select Query ====
	fmt.Println("\nExample 5: Custom Select with Count")
	fmt.Println("- Base query: SELECT COUNT(*) FROM users")
	
	// Create a custom select query
	customSelect := goqu.Select(goqu.COUNT("*")).From("users")
	
	query5, err := db.NewGridQuery(db.SearchNone, 0, 0)
	if err != nil {
		log.Fatal("Error creating grid query: ", err)
	}
	
	query5.FilterFields = map[string]any{
		"active": "yes",
	}

	// Validate the query
	if err := userGrid.ValidQuery(query5); err != nil {
		log.Fatal("Invalid query: ", err)
	}
	
	// Build the SQL with the custom select
	statement5, err := userGrid.Build(customSelect, query5)
	if err != nil {
		log.Fatal("Error building SQL: ", err)
	}
	
	// Convert to SQL
	sql5, args5, err := statement5.ToSQL()
	if err != nil {
		log.Fatal("Error generating SQL: ", err)
	}
	
	fmt.Println("SQL:", sql5)
	fmt.Println("Args:", args5)

	fmt.Println("\nSample completed successfully!")
}