package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// StorageManager provides convenience methods for storage tier and policy management
type StorageManager struct {
	manager *Manager
}

// NewStorageManager creates a new storage manager
func NewStorageManager(manager *Manager) *StorageManager {
	return &StorageManager{
		manager: manager,
	}
}

// GetStorageTier returns a storage tier with the given name
func (sm *StorageManager) GetStorageTier(name string) (StorageTierConfig, error) {
	if err := ValidateName(name); err != nil {
		return StorageTierConfig{}, fmt.Errorf("invalid storage tier name: %w", err)
	}
	
	// Check if the storage tier exists
	tier, exists := sm.manager.config.StorageTiers[name]
	if !exists {
		return StorageTierConfig{}, fmt.Errorf("storage tier %s does not exist", name)
	}
	
	return tier, nil
}

// ListStorageTiers returns a list of all storage tiers
func (sm *StorageManager) ListStorageTiers() []StorageTierConfig {
	tiers := make([]StorageTierConfig, 0, len(sm.manager.config.StorageTiers))
	for _, tier := range sm.manager.config.StorageTiers {
		tiers = append(tiers, tier)
	}
	return tiers
}

// GetStoragePolicy returns a storage policy with the given name
func (sm *StorageManager) GetStoragePolicy(name string) (StoragePolicyConfig, error) {
	if err := ValidateName(name); err != nil {
		return StoragePolicyConfig{}, fmt.Errorf("invalid storage policy name: %w", err)
	}
	
	// Check if the storage policy exists
	policy, exists := sm.manager.config.StoragePolicies[name]
	if !exists {
		return StoragePolicyConfig{}, fmt.Errorf("storage policy %s does not exist", name)
	}
	
	return policy, nil
}

// ListStoragePolicies returns a list of all storage policies
func (sm *StorageManager) ListStoragePolicies() []StoragePolicyConfig {
	policies := make([]StoragePolicyConfig, 0, len(sm.manager.config.StoragePolicies))
	for _, policy := range sm.manager.config.StoragePolicies {
		policies = append(policies, policy)
	}
	return policies
}

// GetStorageTierUsage returns disk usage information for a storage tier
func (sm *StorageManager) GetStorageTierUsage(ctx context.Context, tierName string) (map[string]interface{}, error) {
	if err := ValidateName(tierName); err != nil {
		return nil, fmt.Errorf("invalid storage tier name: %w", err)
	}
	
	// Check if the storage tier exists
	tier, exists := sm.manager.config.StorageTiers[tierName]
	if !exists {
		return nil, fmt.Errorf("storage tier %s does not exist", tierName)
	}
	
	// Query disk usage from the server
	query := `
		SELECT 
			name,
			path,
			free_space,
			total_space,
			keep_free_space
		FROM system.disks
		WHERE name = ?
	`
	
	var (
		name, path string
		freeSpace, totalSpace, keepFreeSpace uint64
	)
	
	err := sm.manager.client.Conn.QueryRowContext(ctx, query, tierName).Scan(
		&name, &path, &freeSpace, &totalSpace, &keepFreeSpace)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage tier usage: %w", err)
	}
	
	// Calculate used space and usage percentage
	usedSpace := totalSpace - freeSpace
	usagePercent := float64(usedSpace) / float64(totalSpace) * 100
	
	return map[string]interface{}{
		"name":             name,
		"path":             path,
		"free_space_bytes": freeSpace,
		"total_space_bytes": totalSpace,
		"used_space_bytes": usedSpace,
		"usage_percent":    usagePercent,
		"keep_free_space_bytes": keepFreeSpace,
		"disk_type":        tier.DiskType,
	}, nil
}

// GetDatabaseStorageUsage returns storage usage information for a database
func (sm *StorageManager) GetDatabaseStorageUsage(ctx context.Context, databaseName string) (map[string]interface{}, error) {
	if err := ValidateName(databaseName); err != nil {
		return nil, fmt.Errorf("invalid database name: %w", err)
	}
	
	// Check if the database exists
	_, exists := sm.manager.config.Databases[databaseName]
	if !exists {
		return nil, fmt.Errorf("database %s does not exist", databaseName)
	}
	
	// Query storage usage from the server
	query := `
		SELECT 
			database,
			SUM(data_compressed_bytes) AS compressed,
			SUM(data_uncompressed_bytes) AS uncompressed,
			SUM(rows) AS total_rows,
			COUNT() AS total_tables
		FROM system.parts
		WHERE active AND database = ?
		GROUP BY database
	`
	
	var (
		database string
		compressed, uncompressed uint64
		totalRows, totalTables uint64
	)
	
	err := sm.manager.client.Conn.QueryRowContext(ctx, query, databaseName).Scan(
		&database, &compressed, &uncompressed, &totalRows, &totalTables)
	if err != nil {
		// If no parts exist yet, return empty stats
		return map[string]interface{}{
			"database":       databaseName,
			"compressed_bytes": uint64(0),
			"uncompressed_bytes": uint64(0),
			"compression_ratio": float64(1.0),
			"total_rows":     uint64(0),
			"total_tables":   uint64(0),
		}, nil
	}
	
	// Calculate compression ratio
	var compressionRatio float64 = 1.0
	if uncompressed > 0 {
		compressionRatio = float64(compressed) / float64(uncompressed)
	}
	
	return map[string]interface{}{
		"database":       database,
		"compressed_bytes": compressed,
		"uncompressed_bytes": uncompressed,
		"compression_ratio": compressionRatio,
		"total_rows":     totalRows,
		"total_tables":   totalTables,
	}, nil
}

// GenerateStorageConfig generates ClickHouse storage configuration XML
func (sm *StorageManager) GenerateStorageConfig() (string, error) {
	var config strings.Builder
	
	config.WriteString("<yandex>\n")
	config.WriteString("  <storage_configuration>\n")
	
	// Add disks
	config.WriteString("    <disks>\n")
	for _, tier := range sm.manager.config.StorageTiers {
		if tier.Type == "disk" {
			config.WriteString(fmt.Sprintf("      <%s>\n", tier.Name))
			config.WriteString(fmt.Sprintf("        <type>%s</type>\n", tier.DiskType))
			config.WriteString(fmt.Sprintf("        <path>%s</path>\n", tier.Path))
			if tier.MaxDiskUsePercentage > 0 {
				config.WriteString(fmt.Sprintf("        <max_data_part_size_bytes>%d</max_data_part_size_bytes>\n", tier.MaxDataPartSizeBytes))
			}
			if tier.MaxDiskUsePercentage > 0 {
				config.WriteString(fmt.Sprintf("        <max_disk_use_percentage>%d</max_disk_use_percentage>\n", tier.MaxDiskUsePercentage))
			}
			config.WriteString(fmt.Sprintf("      </%s>\n", tier.Name))
		}
	}
	config.WriteString("    </disks>\n")
	
	// Add policies
	config.WriteString("    <policies>\n")
	for _, policy := range sm.manager.config.StoragePolicies {
		config.WriteString(fmt.Sprintf("      <%s>\n", policy.Name))
		
		// Add volumes
		for _, volume := range policy.Volumes {
			config.WriteString(fmt.Sprintf("        <%s>\n", volume.Name))
			
			// Add disks
			config.WriteString("          <disks>\n")
			for _, disk := range volume.Disks {
				config.WriteString(fmt.Sprintf("            <disk>%s</disk>\n", disk))
			}
			config.WriteString("          </disks>\n")
			
			if volume.MaxDataPartSizeBytes > 0 {
				config.WriteString(fmt.Sprintf("          <max_data_part_size_bytes>%d</max_data_part_size_bytes>\n", volume.MaxDataPartSizeBytes))
			}
			
			if volume.PreferNotToMerge {
				config.WriteString("          <prefer_not_to_merge>true</prefer_not_to_merge>\n")
			}
			
			config.WriteString(fmt.Sprintf("        </%s>\n", volume.Name))
		}
		
		config.WriteString(fmt.Sprintf("      </%s>\n", policy.Name))
	}
	config.WriteString("    </policies>\n")
	
	config.WriteString("  </storage_configuration>\n")
	config.WriteString("</yandex>\n")
	
	return config.String(), nil
}

// SaveStorageConfig saves the storage configuration to a file
func (sm *StorageManager) SaveStorageConfig(filePath string) error {
	// Generate the configuration
	config, err := sm.GenerateStorageConfig()
	if err != nil {
		return err
	}
	
	// Create the directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Write the configuration to the file
	if err := ioutil.WriteFile(filePath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}
	
	return nil
}