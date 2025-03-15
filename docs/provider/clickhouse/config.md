# ClickHouse Configuration Management

This document provides information about the ClickHouse configuration management library in Blueprint.

## Overview

The ClickHouse configuration management library provides a convenient way to manage ClickHouse server configuration, including:

- User management (create, update, delete users)
- Storage tier and policy management
- Database configuration
- Profile, quota, and role management
- Configuration export and import
- Storage usage monitoring

## Components

The library consists of the following components:

- **Manager**: The main configuration manager that loads, saves, and applies configuration to a ClickHouse server
- **UserManager**: A specialized manager for user-related operations
- **StorageManager**: A specialized manager for storage-related operations
- **Configuration Types**: Structs that represent various ClickHouse configuration entities

## Usage

### Creating a Manager

```go
import (
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/provider/clickhouse"
    "github.com/oddbit-project/blueprint/provider/clickhouse/config"
)

// Create a ClickHouse client
clickhouseConfig := clickhouse.ClientConfig{
    DSN: "clickhouse://localhost:9000/default?username=default&password=",
}

client, err := clickhouse.NewClient(clickhouseConfig)
if err != nil {
    // Handle error
}

// Create a logger
logger := log.NewLogger()

// Create the configuration manager
manager := config.NewManager(client, logger)

// Load configuration from ClickHouse
err = manager.LoadConfig(context.Background())
if err != nil {
    // Handle error
}
```

### Managing Users

```go
// Create a user manager
userManager := config.NewUserManager(manager)

// Create a new user
user := config.UserConfig{
    Name:     "myuser",
    Password: "mypassword",
    Profile:  "default",
    Networks: []string{"127.0.0.1", "::1"},
}

err = userManager.CreateUser(context.Background(), user)
if err != nil {
    // Handle error
}

// Update a user
user.Password = "newpassword"
err = userManager.UpdateUser(context.Background(), user)
if err != nil {
    // Handle error
}

// Delete a user
err = userManager.DeleteUser(context.Background(), "myuser")
if err != nil {
    // Handle error
}

// List users
users := userManager.ListUsers()
for _, u := range users {
    fmt.Println(u.Name)
}
```

### Managing Storage

```go
// Create a storage manager
storageManager := config.NewStorageManager(manager)

// List storage tiers
tiers := storageManager.ListStorageTiers()
for _, tier := range tiers {
    fmt.Println(tier.Name, tier.Path)
}

// List storage policies
policies := storageManager.ListStoragePolicies()
for _, policy := range policies {
    fmt.Println(policy.Name)
}

// Get storage tier usage
usage, err := storageManager.GetStorageTierUsage(context.Background(), "default")
if err != nil {
    // Handle error
}
fmt.Printf("Used: %.2f GB, Free: %.2f GB\n", 
    float64(usage["used_space_bytes"].(uint64))/1024/1024/1024,
    float64(usage["free_space_bytes"].(uint64))/1024/1024/1024)

// Generate storage configuration XML
xml, err := storageManager.GenerateStorageConfig()
if err != nil {
    // Handle error
}
fmt.Println(xml)
```

### Exporting and Importing Configuration

```go
// Export configuration to JSON
data, err := manager.ExportConfig()
if err != nil {
    // Handle error
}

// Write to file
err = os.WriteFile("clickhouse_config.json", data, 0644)
if err != nil {
    // Handle error
}

// Import configuration from JSON
data, err = os.ReadFile("clickhouse_config.json")
if err != nil {
    // Handle error
}

err = manager.ImportConfig(data)
if err != nil {
    // Handle error
}

// Apply imported configuration to ClickHouse
err = manager.ApplyConfig(context.Background())
if err != nil {
    // Handle error
}
```

## Configuration Types

The library defines various configuration types to represent ClickHouse entities:

### UserConfig

Represents a ClickHouse user with properties like password, profile, quota, network restrictions, and allowed databases.

### StorageTierConfig

Represents a storage tier (disk) in ClickHouse with properties like path, type, and usage limits.

### StoragePolicyConfig

Represents a storage policy in ClickHouse, which defines how data is distributed across storage tiers.

### ProfileConfig

Represents a settings profile that can be assigned to users.

### QuotaConfig

Represents usage quotas that can be enforced on users.

### DatabaseConfig

Represents a database with properties like engine and storage policy.

### RoleConfig

Represents a role with associated permissions and settings.

## Command Line Tool

A command-line tool is provided in `sample/clickhouse_config/main.go` that demonstrates how to use the library.

You can run it with various actions:

```sh
# List all configuration
go run sample/clickhouse_config/main.go --dsn "clickhouse://localhost:9000/default?username=default&password="

# Export configuration to a file
go run sample/clickhouse_config/main.go --action export --file config.json

# Import configuration from a file
go run sample/clickhouse_config/main.go --action import --file config.json

# Add a new user
go run sample/clickhouse_config/main.go --action add-user --user testuser --pass testpass

# Generate storage XML configuration
go run sample/clickhouse_config/main.go --action storage-xml --file storage.xml

# Show storage usage
go run sample/clickhouse_config/main.go --action usage --tier default
```

## Limitations

- Some operations require direct modification of ClickHouse configuration files and cannot be performed through SQL queries (e.g., adding new storage tiers).
- The library manages configuration but does not handle schema management or data manipulation.
- Older versions of ClickHouse may not support all features (e.g., roles).

## Security Considerations

- User passwords are not encrypted in the configuration file export. Handle exported files securely.
- Always use secure connections when connecting to production ClickHouse servers.
- Limit access to the configuration tool to authorized administrators only.