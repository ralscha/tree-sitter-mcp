package tools

import (
	"path/filepath"
	"testing"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
)

func TestGetParseDiagnosticsReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "broken.go")
	mustWrite(t, badFile, "package main\nfunc main( {\n")

	project := &models.Project{RootPath: dir}
	registry := language.NewRegistry()
	treeCache := cache.NewTreeCache(50, 60)
	defer registry.Close()
	defer treeCache.Close()

	diagnostics, err := GetParseDiagnostics(project, "broken.go", registry, treeCache, 20, true)
	if err != nil {
		t.Fatalf("GetParseDiagnostics failed: %v", err)
	}
	if !diagnostics.HasErrors {
		t.Fatal("HasErrors = false, want true")
	}
	if diagnostics.IssueCount == 0 {
		t.Fatal("IssueCount = 0, want > 0")
	}
	if diagnostics.ErrorCount == 0 && diagnostics.MissingCount == 0 {
		t.Fatalf("ErrorCount=%d MissingCount=%d, want at least one issue", diagnostics.ErrorCount, diagnostics.MissingCount)
	}
	if len(diagnostics.Issues) == 0 {
		t.Fatal("Issues should include at least one entry")
	}
	if diagnostics.Issues[0].NodeKind == "" {
		t.Fatal("issue NodeKind should not be empty")
	}
}

func TestGetParseDiagnosticsValidFileNoErrors(t *testing.T) {
	dir := t.TempDir()
	goodFile := filepath.Join(dir, "ok.go")
	mustWrite(t, goodFile, "package main\nfunc main() {}\n")

	project := &models.Project{RootPath: dir}
	registry := language.NewRegistry()
	treeCache := cache.NewTreeCache(50, 60)
	defer registry.Close()
	defer treeCache.Close()

	diagnostics, err := GetParseDiagnostics(project, "ok.go", registry, treeCache, 10, false)
	if err != nil {
		t.Fatalf("GetParseDiagnostics failed: %v", err)
	}
	if diagnostics.HasErrors {
		t.Fatal("HasErrors = true, want false")
	}
	if diagnostics.IssueCount != 0 {
		t.Fatalf("IssueCount = %d, want 0", diagnostics.IssueCount)
	}
	if diagnostics.ErrorCount != 0 || diagnostics.MissingCount != 0 {
		t.Fatalf("ErrorCount=%d MissingCount=%d, want 0/0", diagnostics.ErrorCount, diagnostics.MissingCount)
	}
}
