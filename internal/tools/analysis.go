// Package tools provides code analysis utilities for the MCP server.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
)

// SymbolInfo represents an extracted symbol.
type SymbolInfo struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Location map[string]uint `json:"location,omitempty"`
	Text     string          `json:"text,omitempty"`
}

// ExtractSymbols extracts symbols from a file.
func ExtractSymbols(
	project *models.Project,
	filePath string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
	symbolTypes []string,
) (map[string][]SymbolInfo, error) {
	absPath, err := project.ResolveFilePath(filePath)
	if err != nil {
		return nil, err
	}

	lang := langReg.LanguageForFile(filePath)
	if lang == "" {
		return nil, fmt.Errorf("could not detect language for %s", filePath)
	}

	if symbolTypes == nil {
		symbolTypes = defaultSymbolTypes(lang)
	}

	queries := make(map[string]string)
	for _, st := range symbolTypes {
		tmpl := language.GetQueryTemplate(lang, st)
		if tmpl != "" {
			queries[st] = tmpl
		}
	}

	if len(queries) == 0 {
		return nil, fmt.Errorf("no query templates available for %s and %v", lang, symbolTypes)
	}

	tree, sourceBytes, err := ParseFile(absPath, lang, langReg, treeCache)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	langObj, err := langReg.GetLanguage(lang)
	if err != nil {
		return nil, err
	}

	symbols := make(map[string][]SymbolInfo)

	for symbolType, queryStr := range queries {
		query, qerr := sitter.NewQuery(langObj, queryStr)
		if qerr != nil {
			return nil, fmt.Errorf("invalid %s symbol query for %s: %w", symbolType, lang, *qerr)
		}

		qc := sitter.NewQueryCursor()
		matches := qc.Matches(query, tree.RootNode(), sourceBytes)
		captureNames := query.CaptureNames()
		seen := make(map[string]bool)

		for {
			match := matches.Next()
			if match == nil {
				break
			}

			selected := selectSymbolCaptures(match.Captures, captureNames, symbolType)
			for _, cap := range selected {
				captureName := captureNames[cap.Index]
				name := strings.Trim(cap.Node.Utf8Text(sourceBytes), "\"'")

				sp := cap.Node.StartPosition()
				ep := cap.Node.EndPosition()
				key := fmt.Sprintf("%s:%d:%d:%d:%d", name, sp.Row, sp.Column, ep.Row, ep.Column)
				if seen[key] {
					continue
				}
				seen[key] = true

				symbols[symbolType] = append(symbols[symbolType], SymbolInfo{
					Name: name,
					Type: symbolType,
					Text: captureName,
					Location: map[string]uint{
						"start_row":    sp.Row,
						"start_column": sp.Column,
						"end_row":      ep.Row,
						"end_column":   ep.Column,
					},
				})
			}
		}
		qc.Close()
		query.Close()
	}

	return symbols, nil
}

func selectSymbolCaptures(captures []sitter.QueryCapture, names []string, symbolType string) []sitter.QueryCapture {
	preferredSuffix := ".name"
	for _, capture := range captures {
		if strings.HasSuffix(names[capture.Index], preferredSuffix) {
			var selected []sitter.QueryCapture
			for _, candidate := range captures {
				if strings.HasSuffix(names[candidate.Index], preferredSuffix) {
					selected = append(selected, candidate)
				}
			}
			return selected
		}
	}

	if symbolType == "imports" {
		var selected []sitter.QueryCapture
		for _, capture := range captures {
			if strings.Contains(names[capture.Index], ".") {
				selected = append(selected, capture)
			}
		}
		if len(selected) > 0 {
			return selected
		}
	}

	for _, capture := range captures {
		if strings.Contains(names[capture.Index], ".") {
			return []sitter.QueryCapture{capture}
		}
	}
	return captures
}

func defaultSymbolTypes(lang string) []string {
	switch lang {
	case "rust":
		return []string{"functions", "structs", "enums", "traits", "impls", "imports"}
	case "go":
		return []string{"functions", "structs", "interfaces", "imports"}
	case "c":
		return []string{"functions", "structs", "imports"}
	case "cpp":
		return []string{"functions", "classes", "structs", "imports"}
	case "typescript", "java", "kotlin":
		return []string{"functions", "classes", "interfaces", "imports"}
	case "swift":
		return []string{"functions", "classes", "structs", "imports"}
	case "dart":
		return []string{"functions", "classes", "mixins", "enums", "imports"}
	case "xml":
		return []string{"elements", "attributes"}
	default:
		return []string{"functions", "classes", "imports"}
	}
}

// DependencyInfo holds dependency information for a file.
type DependencyInfo map[string][]string

// FindDependencies finds imports/includes of a file.
func FindDependencies(
	project *models.Project,
	filePath string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
) (DependencyInfo, error) {
	symbols, err := ExtractSymbols(project, filePath, langReg, treeCache, []string{"imports"})
	if err != nil {
		return nil, err
	}

	result := make(DependencyInfo)
	imports := make([]string, 0)
	for _, sym := range symbols["imports"] {
		imports = append(imports, sym.Name)
	}
	result["imports"] = imports
	return result, nil
}

// ComplexityInfo holds code complexity metrics.
type ComplexityInfo struct {
	File          string `json:"file"`
	TotalLines    int    `json:"total_lines"`
	FunctionCount int    `json:"function_count"`
	AvgLength     int    `json:"avg_function_length,omitempty"`
}

// AnalyzeComplexity analyzes code complexity for a file.
func AnalyzeComplexity(
	project *models.Project,
	filePath string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
) (*ComplexityInfo, error) {
	absPath, err := project.ResolveFilePath(filePath)
	if err != nil {
		return nil, err
	}
	if err := checkFileAllowed(absPath); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Clean(absPath))
	if err != nil {
		return nil, err
	}

	totalLines := countLines(data)

	symbols, err := ExtractSymbols(project, filePath, langReg, treeCache, []string{"functions"})
	if err != nil {
		return nil, err
	}

	funcs := symbols["functions"]
	totalFuncLen := 0
	for _, f := range funcs {
		endRow := uint(0)
		startRow := uint(0)
		if loc, ok := f.Location["start_row"]; ok {
			startRow = loc
		}
		if loc, ok := f.Location["end_row"]; ok {
			endRow = loc
		}
		totalFuncLen += int(endRow-startRow) + 1
	}

	avgLen := 0
	if len(funcs) > 0 {
		avgLen = totalFuncLen / len(funcs)
	}

	return &ComplexityInfo{
		File:          filePath,
		TotalLines:    totalLines,
		FunctionCount: len(funcs),
		AvgLength:     avgLen,
	}, nil
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := strings.Count(string(data), "\n")
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

// ProjectAnalysis holds overall project structure analysis.
type ProjectAnalysis struct {
	ProjectName string         `json:"project_name"`
	TotalFiles  int            `json:"total_files"`
	Languages   map[string]int `json:"languages"`
	TopFiles    []string       `json:"top_files,omitempty"`
}

// AnalyzeProjectStructure analyzes the overall structure of a project.
func AnalyzeProjectStructure(
	project *models.Project,
	langReg *language.Registry,
	scanDepth int,
	excludedDirs []string,
) (*ProjectAnalysis, error) {
	if len(excludedDirs) == 0 {
		excludedDirs = []string{".git", "node_modules", "__pycache__", ".venv", "venv", ".tox"}
	}
	filter := NewProjectPathFilter(project.RootPath, excludedDirs)

	langCounts := make(map[string]int)
	err := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr
		}
		if info.IsDir() {
			depth := scanDepth
			if depth < 0 {
				depth = -1
			}
			if filter.ShouldSkipDir(path, info, &depth) {
				return filepath.SkipDir
			}
			return nil
		}
		if filter.ShouldSkipFile(path, info) {
			return nil
		}
		base := filepath.Base(path)
		lang := langReg.LanguageForFile(base)
		if lang != "" {
			langCounts[lang]++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	totalFiles := 0
	for _, count := range langCounts {
		totalFiles += count
	}

	var topFiles []string
	entries, err := os.ReadDir(project.RootPath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				topFiles = append(topFiles, entry.Name())
			}
		}
	}

	return &ProjectAnalysis{
		ProjectName: project.Name,
		TotalFiles:  totalFiles,
		Languages:   langCounts,
		TopFiles:    topFiles,
	}, nil
}

// SimilarCodeMatch represents a match from similarity detection.
type SimilarCodeMatch struct {
	File        string  `json:"file"`
	Similarity  float64 `json:"similarity"`
	Fingerprint string  `json:"fingerprint,omitempty"`
}

// FindSimilarCode finds structurally similar code using AST fingerprinting.
// It computes structure fingerprints for the target file and compares them against
// other files in the project of the same language.
func FindSimilarCode(
	project *models.Project,
	filePath string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
	maxResults int,
	minSimilarity float64,
	excludedDirs []string,
) ([]SimilarCodeMatch, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if minSimilarity < 0 || minSimilarity > 1 {
		return nil, fmt.Errorf("min_similarity must be between 0 and 1")
	}

	absPath, err := project.ResolveFilePath(filePath)
	if err != nil {
		return nil, err
	}
	lang := langReg.LanguageForFile(filePath)
	if lang == "" {
		return nil, fmt.Errorf("could not detect language for %s", filePath)
	}

	// Generate fingerprint for the target file.
	targetFingerprint, err := generateFingerprint(absPath, lang, langReg, treeCache)
	if err != nil {
		return nil, fmt.Errorf("fingerprinting target file: %w", err)
	}

	// Walk the project and compare fingerprints.
	results := make([]SimilarCodeMatch, 0)
	filter := NewProjectPathFilter(project.RootPath, excludedDirs)
	walkErr := filepath.Walk(project.RootPath, func(path string, info os.FileInfo, err error) error {
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

		if sameFilePath(path, absPath) {
			return nil // Skip self.
		}

		// Only compare same-language files.
		fLang := langReg.LanguageForFile(relPath)
		if fLang != lang {
			return nil
		}

		fp, fpErr := generateFingerprint(path, lang, langReg, treeCache)
		if fpErr != nil || len(fp) == 0 || len(targetFingerprint) == 0 {
			return nil //nolint:nilerr
		}

		similarity := jaccardSimilarity(targetFingerprint, fp)
		if similarity >= minSimilarity {
			results = append(results, SimilarCodeMatch{
				File:       relPath,
				Similarity: similarity,
			})
		}

		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Similarity == results[j].Similarity {
			return results[i].File < results[j].File
		}
		return results[i].Similarity > results[j].Similarity
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}

func sameFilePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if filepath.Separator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// generateFingerprint creates an AST structure fingerprint for a file.
// Uses the sequence of node type names as a structural fingerprint.
func generateFingerprint(
	filePath string,
	lang string,
	langReg *language.Registry,
	treeCache *cache.TreeCache,
) ([]string, error) {
	tree, _, err := ParseFile(filePath, lang, langReg, treeCache)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var fingerprint []string
	collectNodeTypes(tree.RootNode(), &fingerprint, 3) // depth-limited for performance
	return fingerprint, nil
}

// collectNodeTypes recursively collects node type names up to maxDepth.
func collectNodeTypes(node *sitter.Node, types *[]string, maxDepth int) {
	if node == nil || maxDepth <= 0 {
		return
	}

	*types = append(*types, node.Kind())

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			collectNodeTypes(child, types, maxDepth-1)
		}
	}
}

// jaccardSimilarity computes the Jaccard similarity coefficient between two string slices.
func jaccardSimilarity(a, b []string) float64 {
	setA := make(map[string]int)
	for _, s := range a {
		setA[s]++
	}

	setB := make(map[string]int)
	for _, s := range b {
		setB[s]++
	}

	intersection := 0
	for k := range setA {
		if countB, ok := setB[k]; ok {
			if setA[k] < countB {
				intersection += setA[k]
			} else {
				intersection += countB
			}
		}
	}

	union := 0
	for _, v := range setA {
		union += v
	}
	for k, v := range setB {
		if _, ok := setA[k]; !ok {
			union += v
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
