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
	files, err := ListProjectFiles(project, "src/*.go", &maxDepth, nil, nil)
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

func TestListProjectFilesSupportsRecursiveGlobAndDottedExtension(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "root.go"), "package root\n")
	mustWrite(t, filepath.Join(dir, "src", "app.go"), "package src\n")
	mustWrite(t, filepath.Join(dir, "src", "nested", "app.go"), "package nested\n")
	mustWrite(t, filepath.Join(dir, "src", "nested", "note.txt"), "text\n")

	project := &models.Project{RootPath: dir}
	files, err := ListProjectFiles(project, "src/**/*.go", nil, []string{".go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"src/app.go", "src/nested/app.go"}
	if len(files) != len(want) || files[0] != want[0] || files[1] != want[1] {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
}

func TestGetFileContentUsesOneBasedStartLine(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "lines.txt"), "one\ntwo\nthree")
	project := &models.Project{RootPath: dir}
	maxLines := 1

	content, err := GetFileContent(project, "lines.txt", &maxLines, 2)
	if err != nil {
		t.Fatal(err)
	}
	if content != "two" {
		t.Fatalf("content = %q, want %q", content, "two")
	}

	content, err = GetFileContent(project, "lines.txt", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Fatalf("content beyond EOF = %q, want empty", content)
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
