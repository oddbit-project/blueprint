package migrations

import (
	"embed"
	"github.com/oddbit-project/blueprint/utils"
	"path"
	"path/filepath"
	"sort"
)

const (
	ErrEmptyFolder = utils.Error("Specified folder is empty")
)

type EmbedSource struct {
	fs   embed.FS
	path string
}

func NewEmbedSource(fs embed.FS, path string) (Source, error) {
	return &EmbedSource{
		fs:   fs,
		path: path,
	}, nil
}

// List  sql files (migrations)
func (d *EmbedSource) List() ([]string, error) {
	var files []string

	items, err := d.fs.ReadDir(d.path)
	if err != nil {
		return nil, err
	}
	for _, f := range items {
		if !f.IsDir() && filepath.Ext(f.Name()) == MigrationFileExtension {
			files = append(files, f.Name())
		}
	}

	sort.Strings(files)

	return files, nil
}

// Read a migration from disk
func (d *EmbedSource) Read(name string) (*MigrationRecord, error) {

	if content, err := d.fs.ReadFile(path.Join(d.path, name)); err != nil {
		return nil, err
	} else {
		return LoadMigration(name, content)
	}
}
