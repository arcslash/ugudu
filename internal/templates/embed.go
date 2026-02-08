// Package templates provides embedded default team templates
package templates

import (
	"embed"
	"io/fs"
	"path/filepath"
)

//go:embed defaults/*
var defaultsFS embed.FS

// List returns all available default template names
func List() ([]string, error) {
	entries, err := fs.ReadDir(defaultsFS, "defaults")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			name := e.Name()[:len(e.Name())-5] // Remove .yaml extension
			names = append(names, name)
		}
	}
	return names, nil
}

// Get returns the content of a default template by name
func Get(name string) ([]byte, error) {
	return fs.ReadFile(defaultsFS, "defaults/"+name+".yaml")
}

// GetFS returns the embedded filesystem for advanced usage
func GetFS() fs.FS {
	sub, err := fs.Sub(defaultsFS, "defaults")
	if err != nil {
		return nil
	}
	return sub
}

// Exists checks if a default template exists
func Exists(name string) bool {
	_, err := Get(name)
	return err == nil
}
