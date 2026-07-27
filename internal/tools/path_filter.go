// Package tools provides shared file filtering helpers.
package tools

import (
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// ProjectPathFilter centralizes hidden-dir, excluded-dir, and .gitignore checks.
type ProjectPathFilter struct {
	rootPath string
	excluded map[string]bool
	ignore   *ignore.GitIgnore
}

// NewProjectPathFilter creates a filter for a project root.
func NewProjectPathFilter(rootPath string, excludedDirs []string) *ProjectPathFilter {
	excluded := make(map[string]bool, len(excludedDirs))
	for _, dir := range excludedDirs {
		normalized := strings.TrimSpace(dir)
		if normalized == "" {
			continue
		}
		excluded[normalized] = true
	}

	var gitIgnore *ignore.GitIgnore
	gitIgnorePath := filepath.Join(rootPath, ".gitignore")
	if info, err := os.Stat(gitIgnorePath); err == nil && !info.IsDir() {
		compiled, compileErr := ignore.CompileIgnoreFile(gitIgnorePath)
		if compileErr == nil {
			gitIgnore = compiled
		}
	}

	return &ProjectPathFilter{
		rootPath: rootPath,
		excluded: excluded,
		ignore:   gitIgnore,
	}
}

// ShouldSkipDir determines whether a directory should be skipped during a walk.
func (f *ProjectPathFilter) ShouldSkipDir(path string, info os.FileInfo, maxDepth *int) bool {
	if !info.IsDir() {
		return false
	}

	relPath := f.relativePath(path)
	if relPath == "." {
		return false
	}

	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return true
	}
	if f.excluded[base] {
		return true
	}
	if maxDepth != nil && *maxDepth >= 0 {
		if strings.Count(relPath, "/")+1 > *maxDepth {
			return true
		}
	}

	return f.isIgnored(relPath, true)
}

// ShouldSkipFile determines whether a file should be skipped during a walk.
func (f *ProjectPathFilter) ShouldSkipFile(path string, info os.FileInfo) bool {
	if info.IsDir() {
		return true
	}
	if !isAllowedRegularFile(path, info) {
		return true
	}
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return true
	}
	return f.isIgnored(f.relativePath(path), false)
}

func (f *ProjectPathFilter) relativePath(path string) string {
	relPath, err := filepath.Rel(f.rootPath, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relPath)
}

func (f *ProjectPathFilter) isIgnored(relPath string, isDir bool) bool {
	if f.ignore == nil || relPath == "." || relPath == "" {
		return false
	}

	relPath = filepath.ToSlash(relPath)
	if f.ignore.MatchesPath(relPath) {
		return true
	}
	if isDir {
		return f.ignore.MatchesPath(relPath + "/")
	}
	return false
}
