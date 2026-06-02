package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFilePathRejectsEscapes(t *testing.T) {
	dir := t.TempDir()
	project := &Project{RootPath: dir}

	if _, err := project.ResolveFilePath("sub/file.go"); err != nil {
		t.Fatalf("ResolveFilePath valid path failed: %v", err)
	}

	if _, err := project.ResolveFilePath("../outside.go"); err == nil {
		t.Fatal("ResolveFilePath should reject parent-directory escapes")
	}

	abs := filepath.Join(dir, "file.go")
	if _, err := project.ResolveFilePath(abs); err == nil {
		t.Fatal("ResolveFilePath should reject absolute paths")
	}
}

func TestRegisterProjectRequiresDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.go")
	if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	registry := NewProjectRegistry()
	if _, err := registry.RegisterProject("", file, ""); err == nil {
		t.Fatal("RegisterProject should reject non-directory paths")
	}
}
