// Package tools provides search utilities for the MCP server.
package tools

import (
	"bufio"
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

	var results []SearchMatch

	err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		base := filepath.Base(path)
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
		if filePattern != "" && filePattern != "**/*" && filePattern != "*" {
			pattern := filepath.ToSlash(filePattern)
			matchedRel, _ := filepath.Match(pattern, relPath)
			matchedBase, _ := filepath.Match(pattern, base)
			if !matchedRel && !matchedBase {
				return nil
			}
		}

		file, err := os.Open(filepath.Clean(path)) //nolint:gosec // inside Walk callback, path is clean
		if err != nil {
			return nil
		}
		defer func() { _ = file.Close() }()

		scanner := bufio.NewScanner(file)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		for i, line := range lines {
			matched := false
			switch {
			case re != nil:
				matched = re.MatchString(line)
			case wholeWord:
				wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(pattern) + `\b`)
				if !caseSensitive {
					wordRe = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(pattern) + `\b`)
				}
				matched = wordRe.MatchString(line)
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
) ([]QueryResult, error) {
	if maxResults <= 0 {
		maxResults = 100
	}

	var results []QueryResult

	processFile := func(absPath, relPath, detectedLang string) error {
		tree, sourceBytes, err := ParseFile(absPath, detectedLang, langReg, treeCache)
		if err != nil {
			return nil // skip files that can't be parsed
		}

		langObj, err := langReg.GetLanguage(detectedLang)
		if err != nil {
			return nil
		}

		query, qerr := sitter.NewQuery(langObj, queryString)
		if qerr != nil {
			return fmt.Errorf("invalid query: %w", qerr)
		}
		defer query.Close()

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
				captureName := query.CaptureNames()[cap.Index]
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
					return filepath.SkipAll
				}
			}
		}
		return nil
	}

	switch {
	case filePath != "":
		absPath, err := project.ResolveFilePath(filePath)
		if err != nil {
			return nil, err
		}
		if lang == "" {
			lang = langReg.LanguageForFile(filePath)
		}
		if lang == "" {
			return nil, fmt.Errorf("could not detect language for %s", filePath)
		}
		if err := processFile(absPath, filePath, lang); err != nil {
			return nil, err
		}
	case lang != "":
		err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr
			}
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.IsDir() || !isAllowedRegularFile(path, info) {
				return nil
			}
			relPath, _ := filepath.Rel(project.RootPath, path)
			relPath = filepath.ToSlash(relPath)
			if detected := langReg.LanguageForFile(relPath); detected != lang {
				return nil
			}
			if len(results) >= maxResults {
				return filepath.SkipAll
			}
			return processFile(path, relPath, lang)
		})
		if err != nil && !errors.Is(err, filepath.SkipAll) {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("either language or file_path must be provided")
	}

	if len(results) > maxResults {
		results = results[:maxResults]
	}
	return results, nil
}
