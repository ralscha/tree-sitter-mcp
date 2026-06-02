// Package tools provides file operation utilities for the MCP server.
package tools

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tree-sitter-mcp/internal/models"
)

// ListProjectFiles lists files in a project with optional filtering.
func ListProjectFiles(project *models.Project, pattern string, maxDepth *int, extensions []string) ([]string, error) {
	if pattern == "" {
		pattern = "**/*"
	}

	var files []string
	extSet := make(map[string]bool)
	for _, ext := range extensions {
		extSet[strings.ToLower(ext)] = true
	}

	err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr
		}

		base := filepath.Base(path)

		// Skip hidden files and directories.
		if strings.HasPrefix(base, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if !isAllowedRegularFile(path, info) {
			return nil
		}

		relPath, _ := filepath.Rel(project.RootPath, path)
		relPath = filepath.ToSlash(relPath)

		if maxDepth != nil && *maxDepth >= 0 {
			depth := 0
			if relPath != "." {
				depth = strings.Count(relPath, "/")
			}
			if depth > *maxDepth {
				return nil
			}
		}

		// Filter by extensions.
		if len(extSet) > 0 {
			ext := strings.TrimPrefix(filepath.Ext(base), ".")
			if !extSet[strings.ToLower(ext)] {
				return nil
			}
		}

		// Filter by pattern (simple glob).
		if pattern != "**/*" && pattern != "*" {
			matchedRel, _ := filepath.Match(filepath.ToSlash(pattern), relPath)
			matchedBase, _ := filepath.Match(filepath.ToSlash(pattern), base)
			if !matchedRel && !matchedBase {
				return nil
			}
		}

		files = append(files, relPath)
		return nil
	})

	sort.Strings(files)
	return files, err
}

// GetFileContent reads the content of a file in a project.
func GetFileContent(project *models.Project, path string, maxLines *int, startLine int) (string, error) {
	absPath, err := project.ResolveFilePath(path)
	if err != nil {
		return "", err
	}
	if err := checkFileAllowed(absPath); err != nil {
		return "", err
	}

	data, err := os.ReadFile(filepath.Clean(absPath))
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")

	if startLine > 0 && startLine < len(lines) {
		lines = lines[startLine:]
	}

	if maxLines != nil && *maxLines > 0 && *maxLines < len(lines) {
		lines = lines[:*maxLines]
	}

	return strings.Join(lines, "\n"), nil
}

// GetFileInfo returns metadata about a file in a project.
func GetFileInfo(project *models.Project, path string) (map[string]any, error) {
	absPath, err := project.ResolveFilePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":          path,
		"absolute_path": absPath,
		"size":          info.Size(),
		"modified":      info.ModTime().String(),
		"is_dir":        info.IsDir(),
	}, nil
}
