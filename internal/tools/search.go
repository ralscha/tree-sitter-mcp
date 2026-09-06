// Package tools provides search utilities for the MCP server.
package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
)

// SearchMatch represents a single text search match.
type SearchMatch struct {
	File    string        `json:"file"`
	Line    int           `json:"line"`
	Text    string        `json:"text"`
	Context []ContextLine `json:"context"`
}

// ContextLine represents a line of context around a match.
type ContextLine struct {
	Line    int    `json:"line"`
	Text    string `json:"text"`
	IsMatch bool   `json:"is_match"`
}

// SearchText searches for a text pattern in project files.
func SearchText(
	project *models.Project,
	pattern string,
	filePattern string,
	maxResults int,
	caseSensitive bool,
	wholeWord bool,
	useRegex bool,
	contextLines int,
	excludedDirs []string,
) ([]SearchMatch, error) {
	if maxResults <= 0 {
		maxResults = 100
	}

	var re *regexp.Regexp
	var searchStr string

	if useRegex {
		flags := ""
		if !caseSensitive {
			flags = "(?i)"
		}
		if wholeWord {
			pattern = `\b(?:` + pattern + `)\b`
		}
		var err error
		re, err = regexp.Compile(flags + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
	} else {
		searchStr = pattern
		if !caseSensitive {
			searchStr = strings.ToLower(searchStr)
		}
	}
	if wholeWord && !useRegex {
		flags := ""
		if !caseSensitive {
			flags = "(?i)"
		}
		re = regexp.MustCompile(flags + `\b` + regexp.QuoteMeta(pattern) + `\b`)
	}
	if contextLines < 0 {
		contextLines = 0
	}

	results := make([]SearchMatch, 0)
	filter := NewProjectPathFilter(project.RootPath, excludedDirs)

	err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible project entries
		}

		if info.IsDir() {
			if filter.ShouldSkipDir(path, info, nil) {
				return filepath.SkipDir
			}
			return nil
		}
		if filter.ShouldSkipFile(path, info) {
			return nil
		}

		relPath := filter.relativePath(path)
		matchedPath, matchErr := matchProjectPattern(filePattern, relPath)
		if matchErr != nil {
			return fmt.Errorf("invalid file pattern: %w", matchErr)
		}
		if !matchedPath {
			return nil
		}

		data, err := os.ReadFile(filepath.Clean(path)) //nolint:gosec // path is constrained by the project walk
		if err != nil {
			return nil //nolint:nilerr // skip files that become unreadable during the walk
		}
		if len(data) == 0 {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i := range lines {
			lines[i] = strings.TrimSuffix(lines[i], "\r")
		}

		for i, line := range lines {
			matched := false
			switch {
			case re != nil:
				matched = re.MatchString(line)
			case caseSensitive:
				matched = strings.Contains(line, searchStr)
			default:
				matched = strings.Contains(strings.ToLower(line), searchStr)
			}

			if matched {
				match := SearchMatch{
					File: relPath,
					Line: i + 1,
					Text: line,
				}

				start := max(0, i-contextLines)
				end := min(len(lines), i+contextLines+1)
				for ctxI := start; ctxI < end; ctxI++ {
					match.Context = append(match.Context, ContextLine{
						Line:    ctxI + 1,
						Text:    lines[ctxI],
						IsMatch: ctxI == i,
					})
				}

				results = append(results, match)
				if len(results) >= maxResults {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}
	return results, err
}

// QueryResult represents a single tree-sitter query match.
type QueryResult struct {
	File     string          `json:"file"`
	Captures []CaptureResult `json:"captures"`
}

// CaptureResult represents a single capture in a query match.
type CaptureResult struct {
	Name     string          `json:"name"`
	Text     string          `json:"text"`
	Location map[string]uint `json:"location,omitempty"`
}

// RunQuery executes a tree-sitter query on project files.
func RunQuery(
	project *models.Project,
	queryString string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
	filePath string,
	lang string,
	maxResults int,
	captureFilter string,
	compact bool,
	excludedDirs []string,
) ([]QueryResult, error) {
	if maxResults <= 0 {
		maxResults = 100
	}
	if filePath == "" && lang == "" {
		return nil, fmt.Errorf("either language or file_path must be provided")
	}
	if lang == "" {
		lang = langReg.LanguageForFile(filePath)
		if lang == "" {
			return nil, fmt.Errorf("could not detect language for %s", filePath)
		}
	}

	langObj, err := langReg.GetLanguage(lang)
	if err != nil {
		return nil, err
	}
	query, queryErr := sitter.NewQuery(langObj, queryString)
	if queryErr != nil {
		return nil, fmt.Errorf("invalid query: %w", *queryErr)
	}
	defer query.Close()
	captureNames := query.CaptureNames()

	results := make([]QueryResult, 0)

	processFile := func(absPath, relPath string, skipParseErrors bool) (bool, error) {
		tree, sourceBytes, err := ParseFile(absPath, lang, langReg, treeCache)
		if err != nil {
			if skipParseErrors {
				return false, nil
			}
			return false, err
		}
		defer tree.Close()

		qc := sitter.NewQueryCursor()
		defer qc.Close()

		matches := qc.Matches(query, tree.RootNode(), sourceBytes)
		for {
			match := matches.Next()
			if match == nil {
				break
			}

			var captures []CaptureResult
			for _, cap := range match.Captures {
				captureName := captureNames[cap.Index]
				if captureFilter != "" && captureName != captureFilter {
					continue
				}

				text := cap.Node.Utf8Text(sourceBytes)

				if compact {
					captures = append(captures, CaptureResult{
						Name: captureName,
						Text: text,
					})
				} else {
					sp := cap.Node.StartPosition()
					ep := cap.Node.EndPosition()
					captures = append(captures, CaptureResult{
						Name: captureName,
						Text: text,
						Location: map[string]uint{
							"start_row":    sp.Row,
							"start_column": sp.Column,
							"end_row":      ep.Row,
							"end_column":   ep.Column,
						},
					})
				}
			}

			if len(captures) > 0 {
				results = append(results, QueryResult{
					File:     relPath,
					Captures: captures,
				})
				if len(results) >= maxResults {
					return true, nil
				}
			}
		}
		return false, nil
	}

	if filePath != "" {
		absPath, err := project.ResolveFilePath(filePath)
		if err != nil {
			return nil, err
		}
		if _, err := processFile(absPath, filePath, false); err != nil {
			return nil, err
		}
	} else {
		filter := NewProjectPathFilter(project.RootPath, excludedDirs)
		err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr
			}
			if info.IsDir() {
				if filter.ShouldSkipDir(path, info, nil) {
					return filepath.SkipDir
				}
				return nil
			}
			if filter.ShouldSkipFile(path, info) {
				return nil
			}
			relPath := filter.relativePath(path)
			if detected := langReg.LanguageForFile(relPath); detected != lang {
				return nil
			}
			if len(results) >= maxResults {
				return filepath.SkipAll
			}
			stop, processErr := processFile(path, relPath, true)
			if processErr != nil {
				return processErr
			}
			if stop {
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil && !errors.Is(err, filepath.SkipAll) {
			return nil, err
		}
	}

	if len(results) > maxResults {
		results = results[:maxResults]
	}
	return results, nil
}
