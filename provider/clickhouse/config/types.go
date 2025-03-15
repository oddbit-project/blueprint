package config

import (
	"fmt"
	"strings"
	"time"
)

// UserConfig represents a ClickHouse user configuration
type UserConfig struct {
	Name            string            `json:"name"`
	Password        string            `json:"password,omitempty"`
	HashedPassword  string            `json:"hashed_password,omitempty"`
	Networks        []string          `json:"networks,omitempty"`
	Profile         string            `json:"profile,omitempty"`
	Quota           string            `json:"quota,omitempty"`
	Roles           []string          `json:"roles,omitempty"`
	Settings        map[string]string `json:"settings,omitempty"`
	AllowDatabases  []string          `json:"allow_databases,omitempty"`
	DenyDatabases   []string          `json:"deny_databases,omitempty"`
	AllowDictionary []string          `json:"allow_dictionary,omitempty"`
	DenyDictionary  []string          `json:"deny_dictionary,omitempty"`
}

// StorageTierConfig represents a storage tier configuration
type StorageTierConfig struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	DiskType       string `json:"disk_type"`
	Path           string `json:"path"`
	MaxDataPartSizeBytes int64 `json:"max_data_part_size_bytes,omitempty"`
	MaxDiskUsePercentage int   `json:"max_disk_use_percentage,omitempty"`
}

// BackupConfig represents a backup configuration
type BackupConfig struct {
	Name            string        `json:"name"`
	BackupPath      string        `json:"backup_path"`
	Databases       []string      `json:"databases,omitempty"`
	Tables          []string      `json:"tables,omitempty"`
	Schedule        string        `json:"schedule,omitempty"`
	RetentionPeriod time.Duration `json:"retention_period,omitempty"`
}

// ProfileConfig represents a ClickHouse profile configuration
type ProfileConfig struct {
	Name     string            `json:"name"`
	ReadOnly bool              `json:"read_only,omitempty"`
	Settings map[string]string `json:"settings"`
}

// QuotaConfig represents a ClickHouse quota configuration
type QuotaConfig struct {
	Name      string    `json:"name"`
	Intervals []Interval `json:"intervals"`
}

// Interval represents a quota interval
type Interval struct {
	Duration  time.Duration `json:"duration"`
	Queries   int           `json:"queries,omitempty"`
	Errors    int           `json:"errors,omitempty"`
	ResultRows int          `json:"result_rows,omitempty"`
	ReadRows  int           `json:"read_rows,omitempty"`
	ExecutionTime time.Duration `json:"execution_time,omitempty"`
}

// DatabaseConfig represents a ClickHouse database configuration
type DatabaseConfig struct {
	Name           string   `json:"name"`
	Engine         string   `json:"engine"`
	Comment        string   `json:"comment,omitempty"`
	StoragePolicy  string   `json:"storage_policy,omitempty"`
	AllowedUsers   []string `json:"allowed_users,omitempty"`
	AllowedRoles   []string `json:"allowed_roles,omitempty"`
}

// TableConfig represents a ClickHouse table configuration
type TableConfig struct {
	Database      string            `json:"database"`
	Name          string            `json:"name"`
	Engine        string            `json:"engine"`
	Columns       []ColumnConfig    `json:"columns"`
	OrderBy       []string          `json:"order_by,omitempty"`
	PartitionBy   string            `json:"partition_by,omitempty"`
	TTL           string            `json:"ttl,omitempty"`
	StoragePolicy string            `json:"storage_policy,omitempty"`
	Settings      map[string]string `json:"settings,omitempty"`
}

// ColumnConfig represents a ClickHouse column configuration
type ColumnConfig struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Default  string `json:"default,omitempty"`
	Comment  string `json:"comment,omitempty"`
	Codec    string `json:"codec,omitempty"`
	TTL      string `json:"ttl,omitempty"`
}

// RoleConfig represents a ClickHouse role configuration
type RoleConfig struct {
	Name            string   `json:"name"`
	Settings        map[string]string `json:"settings,omitempty"`
	Grants          []string `json:"grants,omitempty"`
}

// StoragePolicyConfig represents a ClickHouse storage policy configuration
type StoragePolicyConfig struct {
	Name   string   `json:"name"`
	Volumes []Volume `json:"volumes"`
}

// Volume represents a storage policy volume
type Volume struct {
	Name        string   `json:"name"`
	Disks       []string `json:"disks"`
	MaxDataPartSizeBytes int64 `json:"max_data_part_size_bytes,omitempty"`
	PreferNotToMerge bool `json:"prefer_not_to_merge,omitempty"`
}

// ClickHouseConfig represents the overall ClickHouse configuration
type ClickHouseConfig struct {
	Users          map[string]UserConfig          `json:"users,omitempty"`
	Profiles       map[string]ProfileConfig       `json:"profiles,omitempty"`
	Quotas         map[string]QuotaConfig         `json:"quotas,omitempty"`
	Databases      map[string]DatabaseConfig      `json:"databases,omitempty"`
	StorageTiers   map[string]StorageTierConfig   `json:"storage_tiers,omitempty"`
	StoragePolicies map[string]StoragePolicyConfig `json:"storage_policies,omitempty"`
	Roles          map[string]RoleConfig          `json:"roles,omitempty"`
	Backups        map[string]BackupConfig        `json:"backups,omitempty"`
}

// NewClickHouseConfig creates a new ClickHouse configuration
func NewClickHouseConfig() *ClickHouseConfig {
	return &ClickHouseConfig{
		Users:          make(map[string]UserConfig),
		Profiles:       make(map[string]ProfileConfig),
		Quotas:         make(map[string]QuotaConfig),
		Databases:      make(map[string]DatabaseConfig),
		StorageTiers:   make(map[string]StorageTierConfig),
		StoragePolicies: make(map[string]StoragePolicyConfig),
		Roles:          make(map[string]RoleConfig),
		Backups:        make(map[string]BackupConfig),
	}
}

// ValidateName checks if a name is valid for ClickHouse identifiers
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if strings.ContainsAny(name, "` ;'\"\t\n\r") {
		return fmt.Errorf("name contains invalid characters")
	}
	return nil
}