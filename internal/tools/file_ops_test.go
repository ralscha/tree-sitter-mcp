package tools

import (
	"os"
	"path/filepath"
	"testing"

	"tree-sitter-mcp/internal/models"
)

func TestListProjectFilesRespectsDepthAndPathPattern(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\n")
	mustWrite(t, filepath.Join(dir, "src", "app.go"), "package src\n")
	mustWrite(t, filepath.Join(dir, "src", "nested", "app.go"), "package nested\n")

	project := &models.Project{RootPath: dir}
	maxDepth := 1
	files, err := ListProjectFiles(project, "src/*.go", &maxDepth, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 || files[0] != "src/app.go" {
		t.Fatalf("files = %#v, want [src/app.go]", files)
	}
}

func TestGetFileContentRejectsPathEscape(t *testing.T) {
	dir := t.TempDir()
	project := &models.Project{RootPath: dir}

	if _, err := GetFileContent(project, "../outside.go", nil, 0); err == nil {
		t.Fatal("GetFileContent should reject paths outside the project")
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
