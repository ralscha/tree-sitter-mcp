// Package tools provides AST operation utilities for the MCP server.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
)

// ParseIssue describes a problematic node discovered during parsing.
type ParseIssue struct {
	Type     string          `json:"type"`
	NodeKind string          `json:"node_kind"`
	Location map[string]uint `json:"location"`
	End      map[string]uint `json:"end"`
	Text     string          `json:"text,omitempty"`
}

// ParseDiagnostics reports parse health for a single file.
type ParseDiagnostics struct {
	File         string       `json:"file"`
	Language     string       `json:"language"`
	HasErrors    bool         `json:"has_errors"`
	ErrorCount   int          `json:"error_count"`
	MissingCount int          `json:"missing_count"`
	IssueCount   int          `json:"issue_count"`
	Issues       []ParseIssue `json:"issues"`
}

// GetFileAST returns the AST for a file as a nested map.
func GetFileAST(
	project *models.Project,
	path string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
	maxDepth *int,
	includeText bool,
) (map[string]any, error) {
	absPath, err := project.ResolveFilePath(path)
	if err != nil {
		return nil, err
	}

	lang := langReg.LanguageForFile(path)
	if lang == "" {
		return nil, fmt.Errorf("could not detect language for %s", path)
	}

	tree, sourceBytes, err := ParseFile(absPath, lang, langReg, treeCache)
	if err != nil {
		return nil, err
	}

	depth := 5
	if maxDepth != nil {
		depth = *maxDepth
	}

	return map[string]any{
		"file":     path,
		"language": lang,
		"tree":     models.NodeToMap(tree.RootNode(), sourceBytes, true, includeText, depth),
	}, nil
}

// ParseFile parses a file using tree-sitter, using cache if available.
func ParseFile(
	filePath string,
	language string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
) (*sitter.Tree, []byte, error) {
	// Check cache first.
	if tree, source, ok := treeCache.Get(filePath, language); ok {
		return tree, source, nil
	}

	if err := checkFileAllowed(filePath); err != nil {
		return nil, nil, err
	}

	sourceBytes, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	parser, err := langReg.GetParser(language)
	if err != nil {
		return nil, nil, fmt.Errorf("getting parser for %s: %w", language, err)
	}
	defer parser.Close()

	tree := parser.ParseWithOptions(func(i int, _ sitter.Point) []byte {
		if i < len(sourceBytes) {
			return sourceBytes[i:]
		}
		return []byte{}
	}, nil, nil)
	if tree == nil {
		return nil, nil, fmt.Errorf("parsing %s returned nil tree", filePath)
	}

	// Cache the result if caching is enabled.
	if treeCache.IsEnabled() {
		treeCache.Put(filePath, language, tree, sourceBytes)
	}

	return tree, sourceBytes, nil
}

// FindNodeAtPos finds the most specific AST node at a given position.
func FindNodeAtPos(root *sitter.Node, row, col int) *sitter.Node {
	return models.FindNodeAtPosition(root, uint(row), uint(col))
}

// GetParseDiagnostics returns parse errors and missing-node diagnostics for a file.
func GetParseDiagnostics(
	project *models.Project,
	path string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
	maxIssues int,
	includeText bool,
) (*ParseDiagnostics, error) {
	absPath, err := project.ResolveFilePath(path)
	if err != nil {
		return nil, err
	}

	lang := langReg.LanguageForFile(path)
	if lang == "" {
		return nil, fmt.Errorf("could not detect language for %s", path)
	}

	tree, sourceBytes, err := ParseFile(absPath, lang, langReg, treeCache)
	if err != nil {
		return nil, err
	}

	if maxIssues <= 0 {
		maxIssues = 100
	}

	diagnostics := &ParseDiagnostics{
		File:      path,
		Language:  lang,
		HasErrors: tree.RootNode().HasError(),
		Issues:    make([]ParseIssue, 0),
	}

	collectParseIssues(tree.RootNode(), sourceBytes, includeText, maxIssues, diagnostics)
	diagnostics.IssueCount = diagnostics.ErrorCount + diagnostics.MissingCount

	return diagnostics, nil
}

func collectParseIssues(
	node *sitter.Node,
	sourceBytes []byte,
	includeText bool,
	maxIssues int,
	diagnostics *ParseDiagnostics,
) {
	if node == nil {
		return
	}

	recordIssue := func(issueType string) {
		if len(diagnostics.Issues) >= maxIssues {
			return
		}
		sp := node.StartPosition()
		ep := node.EndPosition()
		issue := ParseIssue{
			Type:     issueType,
			NodeKind: node.Kind(),
			Location: map[string]uint{
				"row":    sp.Row,
				"column": sp.Column,
			},
			End: map[string]uint{
				"row":    ep.Row,
				"column": ep.Column,
			},
		}
		if includeText && sourceBytes != nil {
			text := strings.TrimSpace(node.Utf8Text(sourceBytes))
			if len(text) > 200 {
				text = text[:200] + "..."
			}
			issue.Text = text
		}
		diagnostics.Issues = append(diagnostics.Issues, issue)
	}

	if node.IsError() {
		diagnostics.ErrorCount++
		recordIssue("error")
	}
	if node.IsMissing() {
		diagnostics.MissingCount++
		recordIssue("missing")
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		collectParseIssues(node.Child(i), sourceBytes, includeText, maxIssues, diagnostics)
	}
}
