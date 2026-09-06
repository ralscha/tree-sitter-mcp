package tools

import (
	"path/filepath"
	"strings"
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

func TestSearchTextFindsMatchAfterLongLinePrefix(t *testing.T) {
	dir := t.TempDir()
	mustWriteSearchFile(t, filepath.Join(dir, "long.txt"), strings.Repeat("x", 70*1024)+" needle\n")
	project := &models.Project{RootPath: dir}

	results, err := SearchText(project, "needle", "**/*", 10, true, false, false, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Line != 1 {
		t.Fatalf("results = %#v, want one match on the long first line", results)
	}
}

func TestRunQuerySingleFileHonorsLimitWithoutError(t *testing.T) {
	dir := t.TempDir()
	mustWriteSearchFile(t, filepath.Join(dir, "main.go"), "package main\nfunc one() {}\nfunc two() {}\n")
	project := &models.Project{RootPath: dir}
	registry := language.NewRegistry()
	treeCache := cache.NewTreeCache(100, 300)
	defer treeCache.Close()
	defer registry.Close()

	results, err := RunQuery(project, "(function_declaration name: (identifier) @name)", registry, treeCache, "main.go", "go", 1, "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
}

func TestRunQueryValidatesQueryBeforeWalkingFiles(t *testing.T) {
	project := &models.Project{RootPath: t.TempDir()}
	registry := language.NewRegistry()
	treeCache := cache.NewTreeCache(100, 300)
	defer treeCache.Close()
	defer registry.Close()

	if _, err := RunQuery(project, "((not-valid", registry, treeCache, "", "go", 10, "", false, nil); err == nil {
		t.Fatal("RunQuery should validate a query even when the project has no matching files")
	}
}

func mustWriteSearchFile(t *testing.T, path string, content string) {
	t.Helper()
	mustWrite(t, path, content)
}
