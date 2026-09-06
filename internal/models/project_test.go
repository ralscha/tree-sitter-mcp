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

	if _, err := project.ResolveFilePath("..config"); err != nil {
		t.Fatalf("ResolveFilePath rejected a valid dot-prefixed filename: %v", err)
	}
}

func TestResolveFilePathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.go")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	project := &Project{RootPath: root}
	if _, err := project.ResolveFilePath("link.go"); err == nil {
		t.Fatal("ResolveFilePath should reject a symlink escaping the project")
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

func TestRegisterProjectRejectsNameCollision(t *testing.T) {
	registry := NewProjectRegistry()
	first := t.TempDir()
	second := t.TempDir()

	if _, err := registry.RegisterProject("same", first, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.RegisterProject("same", second, ""); err == nil {
		t.Fatal("RegisterProject should reject the same name for a different root")
	}
}
