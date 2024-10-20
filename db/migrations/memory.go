package migrations

import (
	"github.com/oddbit-project/blueprint/utils"
	"path/filepath"
	"sort"
)

const (
	ErrFileNotFound = utils.Error("file not found")
)

type MemorySource struct {
	files map[string][]byte
}

func NewMemorySource() *MemorySource {
	return &MemorySource{
		files: make(map[string][]byte, 0),
	}
}

// Add a migration
func (d *MemorySource) Add(name string, content string) {
	d.files[name] = []byte(content)
}

// List  sql files (migrations)
func (d *MemorySource) List() ([]string, error) {
	var files []string
	for k := range d.files {
		if filepath.Ext(k) == MigrationFileExtension {
			files = append(files, k)
		}
	}
	sort.Strings(files)
	return files, nil
}

// Read a migration
func (d *MemorySource) Read(name string) (*MigrationRecord, error) {

	if content, ok := d.files[name]; !ok {
		return nil, ErrFileNotFound
	} else {
		return LoadMigration(name, content)
	}
}
