// Package tools provides pre-parsing utilities for the MCP server.
package tools

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
)

// PreParseResult holds statistics from a pre-parse run.
type PreParseResult struct {
	RootPath    string         `json:"root_path"`
	TotalFiles  int            `json:"total_files"`
	Parsed      int            `json:"parsed"`
	Skipped     int            `json:"skipped"`
	Errors      int            `json:"errors"`
	ByLanguage  map[string]int `json:"by_language"`
	ElapsedSecs float64        `json:"elapsed_seconds"`
}

// PreParseProject walks a directory tree and parses all recognized source files
// using tree-sitter, populating the parse tree cache. Hidden files/dirs and
// excluded directories are skipped.
func PreParseProject(
	rootPath string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
	excludedDirs []string,
) (*PreParseResult, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", rootPath, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", absPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absPath)
	}

	excludedSet := make(map[string]bool, len(excludedDirs))
	for _, d := range excludedDirs {
		excludedSet[d] = true
	}

	result := &PreParseResult{
		RootPath:   absPath,
		ByLanguage: make(map[string]int),
	}

	start := time.Now()

	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible entries
		}

		base := filepath.Base(path)

		// Skip hidden files and directories.
		if strings.HasPrefix(base, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip excluded directories.
		if info.IsDir() && excludedSet[base] {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		result.TotalFiles++
		if !isAllowedRegularFile(path, info) {
			result.Skipped++
			return nil
		}

		// Detect language from extension.
		lang := langReg.LanguageForFile(base)
		if lang == "" {
			result.Skipped++
			return nil
		}

		// Ensure the language parser is available.
		if !langReg.IsLanguageAvailable(lang) {
			result.Skipped++
			return nil
		}

		// Parse the file (ParseFile handles caching internally).
		_, _, parseErr := ParseFile(path, lang, langReg, treeCache)
		if parseErr != nil {
			result.Errors++
			if strings.EqualFold(os.Getenv("TREE_SITTER_MCP_LOG_LEVEL"), "DEBUG") {
				log.Printf("pre-parse error %s: %v\n", path, parseErr)
			}
			return nil
		}

		result.Parsed++
		result.ByLanguage[lang]++
		return nil
	})

	result.ElapsedSecs = time.Since(start).Seconds()

	if err != nil {
		return result, fmt.Errorf("walk error: %w", err)
	}

	return result, nil
}
