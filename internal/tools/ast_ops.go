// Package tools provides AST operation utilities for the MCP server.
package tools

import (
	"fmt"
	"os"
	"path/filepath"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
)

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
