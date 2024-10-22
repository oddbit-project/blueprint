package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/oddbit-project/blueprint/utils"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

const (
	ErrPathNotFound  = utils.Error("Specified migration path not found")
	ErrInvalidPath   = utils.Error("Specified migration path is not a directory")
	ErrInvalidFile   = utils.Error("Specified migration path is not a file")
	ErrReadMigration = utils.Error("Error reading migration")

	MigrationFileExtension = ".sql"
)

type DiskSource struct {
	Path string
}

func NewDiskSource(path string) (Source, error) {
	var err error
	var info os.FileInfo
	if path, err = filepath.Abs(path); err != nil {
		return nil, err
	}

	if info, err = os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPathNotFound
		}
		return nil, ErrInvalidPath
	}

	if !info.IsDir() {
		return nil, ErrInvalidPath
	}

	return &DiskSource{
		Path: path,
	}, nil
}

// List  sql files (migrations)
func (d *DiskSource) List() ([]string, error) {
	var files []string

	if err := filepath.Walk(d.Path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == MigrationFileExtension {
			files = append(files, info.Name())
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// sort names
	sort.Strings(files)

	return files, nil
}

// Read a migration from disk
func (d *DiskSource) Read(name string) (*MigrationRecord, error) {
	basePath, err := filepath.Abs(d.Path)
	if err != nil {
		return nil, err
	}
	fullPath := path.Join(basePath, name)
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return nil, ErrPathNotFound
	}
	if info.IsDir() {
		return nil, ErrInvalidFile
	}

	if content, err := os.ReadFile(fullPath); err != nil {
		return nil, ErrReadMigration
	} else {
		return LoadMigration(name, content)
	}
}

// LoadMigration from []byte slice
func LoadMigration(name string, content []byte) (*MigrationRecord, error) {
	h := sha256.New()
	h.Write(content)
	return &MigrationRecord{
		Created:  time.Now(),
		Name:     name,
		SHA2:     hex.EncodeToString(h.Sum(nil)),
		Contents: string(content),
	}, nil
}
