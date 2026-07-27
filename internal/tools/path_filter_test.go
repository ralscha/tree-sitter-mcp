package tools

import (
	"path/filepath"
	"testing"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
)

func TestGitIgnoreAffectsListSearchAndPreParse(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".gitignore"), "ignored/\n*.gen.go\n")
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\nconst keepNeedle = 1\n")
	mustWrite(t, filepath.Join(dir, "ignored", "skip.go"), "package ignored\nconst keepNeedle = 1\n")
	mustWrite(t, filepath.Join(dir, "generated.gen.go"), "package main\nconst keepNeedle = 1\n")

	project := &models.Project{RootPath: dir}

	files, err := ListProjectFiles(project, "**/*", nil, nil, nil)
	if err != nil {
		t.Fatalf("ListProjectFiles failed: %v", err)
	}
	if len(files) != 1 || files[0] != "main.go" {
		t.Fatalf("files = %#v, want [main.go]", files)
	}

	searchResults, err := SearchText(project, "keepNeedle", "**/*", 20, true, false, false, 0, nil)
	if err != nil {
		t.Fatalf("SearchText failed: %v", err)
	}
	if len(searchResults) != 1 || searchResults[0].File != "main.go" {
		t.Fatalf("searchResults = %#v, want one match in main.go", searchResults)
	}

	registry := language.NewRegistry()
	treeCache := cache.NewTreeCache(50, 60)
	defer registry.Close()
	defer treeCache.Close()

	preParseResult, err := PreParseProject(dir, registry, treeCache, nil)
	if err != nil {
		t.Fatalf("PreParseProject failed: %v", err)
	}
	if preParseResult.Parsed != 1 {
		t.Fatalf("Parsed = %d, want 1", preParseResult.Parsed)
	}
	if preParseResult.ByLanguage["go"] != 1 {
		t.Fatalf("ByLanguage[go] = %d, want 1", preParseResult.ByLanguage["go"])
	}
}
