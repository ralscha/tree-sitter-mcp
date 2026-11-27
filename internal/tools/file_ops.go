// Package tools provides file operation utilities for the MCP server.
package tools

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tree-sitter-mcp/internal/models"
)

// ListProjectFiles lists files in a project with optional filtering.
func ListProjectFiles(project *models.Project, pattern string, maxDepth *int, extensions []string, excludedDirs []string) ([]string, error) {
	if pattern == "" {
		pattern = "**/*"
	}

	files := make([]string, 0)
	filter := NewProjectPathFilter(project.RootPath, excludedDirs)
	extSet := make(map[string]bool)
	for _, ext := range extensions {
		normalized := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
		if normalized != "" {
			extSet[normalized] = true
		}
	}

	err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr
		}

		if info.IsDir() {
			if filter.ShouldSkipDir(path, info, maxDepth) {
				return filepath.SkipDir
			}
			return nil
		}

		if filter.ShouldSkipFile(path, info) {
			return nil
		}

		relPath := filter.relativePath(path)
		base := filepath.Base(path)

		// Filter by extensions.
		if len(extSet) > 0 {
			ext := strings.TrimPrefix(filepath.Ext(base), ".")
			if !extSet[strings.ToLower(ext)] {
				return nil
			}
		}

		matched, matchErr := matchProjectPattern(pattern, relPath)
		if matchErr != nil {
			return matchErr
		}
		if !matched {
			return nil
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

	// Skip directly to the requested byte range. Building a slice for every
	// line is expensive for a file with many short lines.
	for line := 1; line < startLine; line++ {
		next := bytes.IndexByte(data, '\n')
		if next < 0 {
			return "", nil
		}
		data = data[next+1:]
	}
	if maxLines != nil && *maxLines > 0 {
		offset := 0
		for range *maxLines {
			next := bytes.IndexByte(data[offset:], '\n')
			if next < 0 {
				return string(data), nil
			}
			offset += next + 1
		}
		data = data[:offset-1]
	}
	return string(data), nil
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
