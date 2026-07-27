package tools

import (
	"path/filepath"
	"testing"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
)

func TestSearchTextRespectsFilePattern(t *testing.T) {
	dir := t.TempDir()
	mustWriteSearchFile(t, filepath.Join(dir, "main.go"), "package main\nconst needle = 1\n")
	mustWriteSearchFile(t, filepath.Join(dir, "README.md"), "needle\n")

	project := &models.Project{RootPath: dir}
	results, err := SearchText(project, "needle", "*.go", 10, true, false, false, 0, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 || results[0].File != "main.go" {
		t.Fatalf("results = %#v, want one match in main.go", results)
	}
}

func TestRunQueryReturnsInvalidQueryError(t *testing.T) {
	dir := t.TempDir()
	mustWriteSearchFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")

	project := &models.Project{RootPath: dir}
	registry := language.NewRegistry()
	treeCache := cache.NewTreeCache(100, 300)
	defer treeCache.Close()
	defer registry.Close()

	_, err := RunQuery(project, "((not-valid", registry, treeCache, "main.go", "go", 10, "", false, nil)
	if err == nil {
		t.Fatal("RunQuery should return invalid query errors")
	}
}

func mustWriteSearchFile(t *testing.T, path string, content string) {
	t.Helper()
	mustWrite(t, path, content)
}
