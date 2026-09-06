// Package models provides data types for projects, ASTs, and related entities.
package models

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	ignore "github.com/sabhiram/go-gitignore"
)

// ProjectRegistry manages registered projects.
type ProjectRegistry struct {
	mu       sync.RWMutex
	projects map[string]*Project
}

// NewProjectRegistry creates a new project registry.
func NewProjectRegistry() *ProjectRegistry {
	return &ProjectRegistry{
		projects: make(map[string]*Project),
	}
}

// RegisterProject registers a new project or returns an existing one.
func (r *ProjectRegistry) RegisterProject(name, path, description string) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if info, statErr := os.Stat(absPath); statErr != nil {
		return nil, statErr
	} else if !info.IsDir() {
		return nil, fmt.Errorf("project path is not a directory: %s", absPath)
	}
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	if name == "" {
		name = filepath.Base(absPath)
	}

	if existing, ok := r.projects[name]; ok {
		if samePath(existing.RootPath, absPath) {
			return existing, nil
		}
		return nil, fmt.Errorf("project name %q is already registered for %s", name, existing.RootPath)
	}

	p := &Project{
		Name:        name,
		RootPath:    absPath,
		Description: description,
		Languages:   make(map[string]int),
	}
	r.projects[name] = p
	return p, nil
}

// GetProject retrieves a project by name.
func (r *ProjectRegistry) GetProject(name string) (*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.projects[name]
	if !ok {
		return nil, &ProjectError{Message: "project not found: " + name}
	}
	return p, nil
}

// RemoveProject removes a registered project.
func (r *ProjectRegistry) RemoveProject(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.projects[name]; !ok {
		return &ProjectError{Message: "project not found: " + name}
	}
	delete(r.projects, name)
	return nil
}

// ListProjects returns all registered projects as maps.
func (r *ProjectRegistry) ListProjects() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.projects))
	for name := range r.projects {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]map[string]any, 0, len(names))
	for _, name := range names {
		result = append(result, r.projects[name].ToMap())
	}
	return result
}

// Project represents a code project for analysis.
type Project struct {
	Name         string
	RootPath     string
	Description  string
	Languages    map[string]int // language -> file count
	LastScanTime int64
	scanLock     sync.RWMutex
}

// ToMap converts a Project to a map representation.
func (p *Project) ToMap() map[string]any {
	p.scanLock.RLock()
	defer p.scanLock.RUnlock()

	return map[string]any{
		"name":           p.Name,
		"root_path":      p.RootPath,
		"description":    p.Description,
		"languages":      cloneLanguageCounts(p.Languages),
		"last_scan_time": p.LastScanTime,
	}
}

// ScanFiles scans project files and identifies languages.
func (p *Project) ScanFiles(registry LanguageDetector, excludedDirs []string) map[string]int {
	p.scanLock.Lock()
	defer p.scanLock.Unlock()

	// Skip if scanned recently (within 60 seconds)
	if time.Now().Unix()-p.LastScanTime < 60 {
		return cloneLanguageCounts(p.Languages)
	}

	languages := make(map[string]int)
	excluded := make(map[string]bool)
	for _, d := range excludedDirs {
		excluded[d] = true
	}

	var gitIgnore *ignore.GitIgnore
	if gi, err := ignore.CompileIgnoreFile(filepath.Join(p.RootPath, ".gitignore")); err == nil {
		gitIgnore = gi
	}

	_ = filepath.Walk(p.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible files
		}

		// Skip hidden files/dirs, excluded dirs, and .gitignore matches.
		base := filepath.Base(path)
		relPath, relErr := filepath.Rel(p.RootPath, path)
		if relErr != nil {
			relPath = base
		}
		relPath = filepath.ToSlash(relPath)

		if strings.HasPrefix(base, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if gitIgnore != nil && relPath != "." {
			if gitIgnore.MatchesPath(relPath) || (info.IsDir() && gitIgnore.MatchesPath(relPath+"/")) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() && excluded[base] {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		lang := registry.LanguageForFile(base)
		if lang != "" {
			languages[lang]++
		}

		return nil
	})

	p.Languages = languages
	p.LastScanTime = time.Now().Unix()
	return cloneLanguageCounts(languages)
}

func cloneLanguageCounts(languages map[string]int) map[string]int {
	clone := make(map[string]int, len(languages))
	for language, count := range languages {
		clone[language] = count
	}
	return clone
}

// ResolveFilePath returns a clean absolute path inside the project root.
func (p *Project) ResolveFilePath(relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", relativePath)
	}

	root, err := filepath.Abs(p.RootPath)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(root, relativePath))
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return candidate, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes project root: %s", relativePath)
	}

	// Lexical containment is insufficient when a path traverses a symlink.
	// Resolve existing targets and ensure their real path remains in the project.
	realRoot, rootErr := filepath.EvalSymlinks(root)
	realCandidate, candidateErr := filepath.EvalSymlinks(candidate)
	if rootErr == nil && candidateErr == nil {
		realRel, relErr := filepath.Rel(realRoot, realCandidate)
		if relErr != nil || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) || filepath.IsAbs(realRel) {
			return "", fmt.Errorf("path escapes project root through a symlink: %s", relativePath)
		}
	}
	return candidate, nil
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// LanguageDetector is the interface for detecting languages from filenames.
type LanguageDetector interface {
	LanguageForFile(filename string) string
}

// ProjectError represents a project-related error.
type ProjectError struct {
	Message string
}

func (e *ProjectError) Error() string {
	return e.Message
}
