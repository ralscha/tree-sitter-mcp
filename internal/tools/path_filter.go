// Package tools provides shared file filtering helpers.
package tools

import (
	"os"
	pathpkg "path"
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
	// filepath.Walk uses Lstat. Reject links and special files so a project
	// cannot expose a target outside its root or block on a named pipe.
	if !info.Mode().IsRegular() {
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

// matchProjectPattern matches slash-separated project paths. A ** segment
// matches zero or more complete path segments; other segments use Go glob
// syntax. Patterns without a slash are matched against the basename.
func matchProjectPattern(pattern, relPath string) (bool, error) {
	pattern = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(pattern)), "./")
	relPath = strings.TrimPrefix(filepath.ToSlash(relPath), "./")
	if pattern == "" || pattern == "*" || pattern == "**/*" {
		return true, nil
	}

	patternParts := strings.Split(pattern, "/")
	for _, part := range patternParts {
		if part == "**" {
			continue
		}
		if _, err := pathpkg.Match(part, ""); err != nil {
			return false, err
		}
	}
	if !strings.Contains(pattern, "/") {
		return pathpkg.Match(pattern, pathpkg.Base(relPath))
	}

	pathParts := strings.Split(relPath, "/")
	type state struct{ pattern, path int }
	memo := make(map[state]bool)
	seen := make(map[state]bool)
	var match func(int, int) bool
	match = func(patternIndex, pathIndex int) bool {
		key := state{patternIndex, pathIndex}
		if seen[key] {
			return memo[key]
		}
		seen[key] = true

		var matched bool
		switch {
		case patternIndex == len(patternParts):
			matched = pathIndex == len(pathParts)
		case patternParts[patternIndex] == "**":
			matched = match(patternIndex+1, pathIndex) ||
				(pathIndex < len(pathParts) && match(patternIndex, pathIndex+1))
		case pathIndex < len(pathParts):
			segmentMatch, _ := pathpkg.Match(patternParts[patternIndex], pathParts[pathIndex])
			matched = segmentMatch && match(patternIndex+1, pathIndex+1)
		}
		memo[key] = matched
		return matched
	}

	return match(0, 0), nil
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
