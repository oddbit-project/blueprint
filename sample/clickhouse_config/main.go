package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/clickhouse"
	chConfig "github.com/oddbit-project/blueprint/provider/clickhouse/config"
)

func main() {
	// Parse command line flags
	dsn := flag.String("dsn", "clickhouse://127.0.0.1:9000/default?username=default&password=", "ClickHouse DSN")
	action := flag.String("action", "list", "Action to perform: list, export, import, add-user, update-user, delete-user, add-storage-tier, storage-xml, usage")
	// format := flag.String("format", "json", "Export format: json, yaml") // Currently only JSON is supported
	file := flag.String("file", "", "File to import/export configuration from/to")
	userName := flag.String("user", "", "User name for user operations")
	userPass := flag.String("pass", "", "User password for user operations")
	userProfile := flag.String("profile", "", "User profile for user operations")
	tierName := flag.String("tier", "", "Storage tier name for storage operations")
	tierPath := flag.String("path", "", "Storage tier path for storage operations")
	dbName := flag.String("db", "", "Database name for database operations")

	flag.Parse()

	// Initialize logger
	logger := &log.Logger{}

	// Create ClickHouse client
	clickhouseConfig := &clickhouse.ClientConfig{
		DSN: *dsn,
	}

	client, err := clickhouse.NewClient(clickhouseConfig)
	if err != nil {
		fmt.Printf("Error: Failed to create ClickHouse client: %v\n", err)
		os.Exit(1)
	}

	// Create configuration manager
	manager := chConfig.NewManager(client, logger)
	ctx := context.Background()

	// Load configuration from ClickHouse
	if err := manager.LoadConfig(ctx); err != nil {
		fmt.Printf("Error: Failed to load configuration from ClickHouse: %v\n", err)
		os.Exit(1)
	}

	// Create user and storage managers
	userManager := chConfig.NewUserManager(manager)
	storageManager := chConfig.NewStorageManager(manager)

	// Execute requested action
	switch strings.ToLower(*action) {
	case "list":
		// List all configuration
		printConfiguration(manager.GetConfig())

	case "export":
		// Export configuration to a file
		data, err := manager.ExportConfig()
		if err != nil {
			fmt.Printf("Error: Failed to export configuration: %v\n", err)
			os.Exit(1)
		}

		if *file == "" {
			// Output to stdout
			fmt.Println(string(data))
		} else {
			// Write to file
			if err := os.WriteFile(*file, data, 0644); err != nil {
				fmt.Printf("Error: Failed to write configuration to file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Configuration exported successfully to %s\n", *file)
		}

	case "import":
		// Import configuration from a file
		if *file == "" {
			fmt.Println("Error: Import requires --file parameter")
			os.Exit(1)
		}

		data, err := os.ReadFile(*file)
		if err != nil {
			fmt.Printf("Error: Failed to read configuration file %s: %v\n", *file, err)
			os.Exit(1)
		}

		if err := manager.ImportConfig(data); err != nil {
			fmt.Printf("Error: Failed to import configuration: %v\n", err)
			os.Exit(1)
		}

		// Apply the imported configuration
		if err := manager.ApplyConfig(ctx); err != nil {
			fmt.Printf("Error: Failed to apply configuration: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Configuration imported and applied successfully from %s\n", *file)

	case "add-user":
		// Add a new user
		if *userName == "" {
			fmt.Println("Error: User name is required")
			os.Exit(1)
		}

		user := chConfig.UserConfig{
			Name:     *userName,
			Password: *userPass,
		}

		if *userProfile != "" {
			user.Profile = *userProfile
		}

		if err := userManager.CreateUser(ctx, user); err != nil {
			fmt.Printf("Error: Failed to create user %s: %v\n", *userName, err)
			os.Exit(1)
		}

		fmt.Printf("User %s created successfully\n", *userName)

	case "update-user":
		// Update an existing user
		if *userName == "" {
			fmt.Println("Error: User name is required")
			os.Exit(1)
		}

		// Get the existing user
		user, err := userManager.GetUser(*userName)
		if err != nil {
			fmt.Printf("Error: Failed to get user %s: %v\n", *userName, err)
			os.Exit(1)
		}

		// Update the user properties
		if *userPass != "" {
			user.Password = *userPass
		}

		if *userProfile != "" {
			user.Profile = *userProfile
		}

		if err := userManager.UpdateUser(ctx, user); err != nil {
			fmt.Printf("Error: Failed to update user %s: %v\n", *userName, err)
			os.Exit(1)
		}

		fmt.Printf("User %s updated successfully\n", *userName)

	case "delete-user":
		// Delete a user
		if *userName == "" {
			fmt.Println("Error: User name is required")
			os.Exit(1)
		}

		if err := userManager.DeleteUser(ctx, *userName); err != nil {
			fmt.Printf("Error: Failed to delete user %s: %v\n", *userName, err)
			os.Exit(1)
		}

		fmt.Printf("User %s deleted successfully\n", *userName)

	case "add-storage-tier":
		// This operation is informational only since storage tiers can only be added via config files
		if *tierName == "" || *tierPath == "" {
			fmt.Println("Error: Tier name and path are required")
			os.Exit(1)
		}

		tier := chConfig.StorageTierConfig{
			Name:     *tierName,
			Type:     "disk",
			DiskType: "local",
			Path:     *tierPath,
		}

		// We can't directly add a storage tier via SQL, but we can add it to our configuration
		manager.GetConfig().StorageTiers[*tierName] = tier

		fmt.Printf("Storage tier %s added to configuration (requires server config.xml modification)\n", *tierName)

	case "storage-xml":
		// Generate and export storage configuration XML
		xml, err := storageManager.GenerateStorageConfig()
		if err != nil {
			fmt.Printf("Error: Failed to generate storage configuration XML: %v\n", err)
			os.Exit(1)
		}

		if *file == "" {
			// Output to stdout
			fmt.Println(xml)
		} else {
			// Write to file
			if err := os.WriteFile(*file, []byte(xml), 0644); err != nil {
				fmt.Printf("Error: Failed to write storage configuration to file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Storage configuration exported successfully to %s\n", *file)
		}

	case "usage":
		// Display storage usage information
		if *dbName != "" {
			// Get database storage usage
			usage, err := storageManager.GetDatabaseStorageUsage(ctx, *dbName)
			if err != nil {
				fmt.Printf("Error: Failed to get database storage usage for %s: %v\n", *dbName, err)
				os.Exit(1)
			}

			fmt.Printf("Database: %s\n", usage["database"])
			fmt.Printf("Total Tables: %d\n", usage["total_tables"])
			fmt.Printf("Total Rows: %d\n", usage["total_rows"])
			fmt.Printf("Compressed Size: %.2f MB\n", float64(usage["compressed_bytes"].(uint64))/1024/1024)
			fmt.Printf("Uncompressed Size: %.2f MB\n", float64(usage["uncompressed_bytes"].(uint64))/1024/1024)
			fmt.Printf("Compression Ratio: %.2f\n", usage["compression_ratio"])
		} else if *tierName != "" {
			// Get storage tier usage
			usage, err := storageManager.GetStorageTierUsage(ctx, *tierName)
			if err != nil {
				fmt.Printf("Error: Failed to get storage tier usage for %s: %v\n", *tierName, err)
				os.Exit(1)
			}

			fmt.Printf("Storage Tier: %s\n", usage["name"])
			fmt.Printf("Path: %s\n", usage["path"])
			fmt.Printf("Type: %s\n", usage["disk_type"])
			fmt.Printf("Total Space: %.2f GB\n", float64(usage["total_space_bytes"].(uint64))/1024/1024/1024)
			fmt.Printf("Free Space: %.2f GB\n", float64(usage["free_space_bytes"].(uint64))/1024/1024/1024)
			fmt.Printf("Used Space: %.2f GB\n", float64(usage["used_space_bytes"].(uint64))/1024/1024/1024)
			fmt.Printf("Usage: %.2f%%\n", usage["usage_percent"])
		} else {
			fmt.Println("Error: Either --db or --tier parameter is required for usage information")
			os.Exit(1)
		}

	default:
		fmt.Printf("Error: Unknown action %s\n", *action)
		os.Exit(1)
	}
}

// printConfiguration prints the configuration to stdout
func printConfiguration(cfg *chConfig.ClickHouseConfig) {
	fmt.Println("=== ClickHouse Configuration ===")

	fmt.Println("\n--- Users ---")
	for _, user := range cfg.Users {
		fmt.Printf("User: %s\n", user.Name)
		fmt.Printf("  Profile: %s\n", user.Profile)
		fmt.Printf("  Quota: %s\n", user.Quota)
		if len(user.Networks) > 0 {
			fmt.Printf("  Networks: %s\n", strings.Join(user.Networks, ", "))
		}
		if len(user.Roles) > 0 {
			fmt.Printf("  Roles: %s\n", strings.Join(user.Roles, ", "))
		}
		if len(user.AllowDatabases) > 0 {
			fmt.Printf("  Allowed Databases: %s\n", strings.Join(user.AllowDatabases, ", "))
		}
		fmt.Println()
	}

	fmt.Println("\n--- Profiles ---")
	for _, profile := range cfg.Profiles {
		fmt.Printf("Profile: %s\n", profile.Name)
		fmt.Printf("  Read Only: %v\n", profile.ReadOnly)
		if len(profile.Settings) > 0 {
			fmt.Println("  Settings:")
			for key, value := range profile.Settings {
				fmt.Printf("    %s: %s\n", key, value)
			}
		}
		fmt.Println()
	}

	fmt.Println("\n--- Quotas ---")
	for _, quota := range cfg.Quotas {
		fmt.Printf("Quota: %s\n", quota.Name)
		for i, interval := range quota.Intervals {
			fmt.Printf("  Interval %d: %s\n", i+1, interval.Duration)
			if interval.Queries > 0 {
				fmt.Printf("    Max Queries: %d\n", interval.Queries)
			}
			if interval.Errors > 0 {
				fmt.Printf("    Max Errors: %d\n", interval.Errors)
			}
			if interval.ResultRows > 0 {
				fmt.Printf("    Max Result Rows: %d\n", interval.ResultRows)
			}
			if interval.ReadRows > 0 {
				fmt.Printf("    Max Read Rows: %d\n", interval.ReadRows)
			}
			if interval.ExecutionTime > 0 {
				fmt.Printf("    Max Execution Time: %s\n", interval.ExecutionTime)
			}
		}
		fmt.Println()
	}

	fmt.Println("\n--- Databases ---")
	for _, db := range cfg.Databases {
		fmt.Printf("Database: %s\n", db.Name)
		fmt.Printf("  Engine: %s\n", db.Engine)
		if db.StoragePolicy != "" {
			fmt.Printf("  Storage Policy: %s\n", db.StoragePolicy)
		}
		if db.Comment != "" {
			fmt.Printf("  Comment: %s\n", db.Comment)
		}
		if len(db.AllowedUsers) > 0 {
			fmt.Printf("  Allowed Users: %s\n", strings.Join(db.AllowedUsers, ", "))
		}
		if len(db.AllowedRoles) > 0 {
			fmt.Printf("  Allowed Roles: %s\n", strings.Join(db.AllowedRoles, ", "))
		}
		fmt.Println()
	}

	fmt.Println("\n--- Storage Tiers ---")
	for _, tier := range cfg.StorageTiers {
		fmt.Printf("Storage Tier: %s\n", tier.Name)
		fmt.Printf("  Type: %s\n", tier.Type)
		fmt.Printf("  Disk Type: %s\n", tier.DiskType)
		fmt.Printf("  Path: %s\n", tier.Path)
		if tier.MaxDataPartSizeBytes > 0 {
			fmt.Printf("  Max Data Part Size: %.2f MB\n", float64(tier.MaxDataPartSizeBytes)/1024/1024)
		}
		if tier.MaxDiskUsePercentage > 0 {
			fmt.Printf("  Max Disk Use Percentage: %d%%\n", tier.MaxDiskUsePercentage)
		}
		fmt.Println()
	}

	fmt.Println("\n--- Storage Policies ---")
	for _, policy := range cfg.StoragePolicies {
		fmt.Printf("Storage Policy: %s\n", policy.Name)
		for i, vol := range policy.Volumes {
			fmt.Printf("  Volume %d: %s\n", i+1, vol.Name)
			fmt.Printf("    Disks: %s\n", strings.Join(vol.Disks, ", "))
			if vol.MaxDataPartSizeBytes > 0 {
				fmt.Printf("    Max Data Part Size: %.2f MB\n", float64(vol.MaxDataPartSizeBytes)/1024/1024)
			}
			if vol.PreferNotToMerge {
				fmt.Printf("    Prefer Not To Merge: %v\n", vol.PreferNotToMerge)
			}
		}
		fmt.Println()
	}

	fmt.Println("\n--- Roles ---")
	for _, role := range cfg.Roles {
		fmt.Printf("Role: %s\n", role.Name)
		if len(role.Settings) > 0 {
			fmt.Println("  Settings:")
			for key, value := range role.Settings {
				fmt.Printf("    %s: %s\n", key, value)
			}
		}
		if len(role.Grants) > 0 {
			fmt.Println("  Grants:")
			for _, grant := range role.Grants {
				fmt.Printf("    %s\n", grant)
			}
		}
		fmt.Println()
	}
}