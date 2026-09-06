// Package server provides the MCP server setup and tool registration for tree-sitter analysis.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tree-sitter-mcp/internal/config"
	"tree-sitter-mcp/internal/container"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
	"tree-sitter-mcp/internal/tools"
)

type Transport string

const (
	StdioTransport Transport = "stdio"
	HTTPTransport  Transport = "http"
)

type RunOptions struct {
	Transport   Transport
	HTTPAddr    string
	AllowRemote bool
}

// MCPServer wraps the MCP server and its dependencies.
type MCPServer struct {
	srv       *mcp.Server
	container *container.Container
	configMu  sync.Mutex
}

// NewMCPServer creates a new MCP server with all tools registered.
func NewMCPServer() *MCPServer {
	return NewMCPServerWithVersion("0.1.0")
}

// NewMCPServerWithVersion creates a server that reports the supplied build version.
func NewMCPServerWithVersion(version string) *MCPServer {
	ctr := container.NewContainer()

	impl := &mcp.Implementation{
		Name:    "tree_sitter",
		Version: version,
	}

	mcpServer := mcp.NewServer(impl, nil)

	s := &MCPServer{
		srv:       mcpServer,
		container: ctr,
	}

	s.registerTools()
	return s
}

// Run starts the MCP server on stdio.
func (s *MCPServer) Run() error {
	return s.RunWithOptions(context.Background(), RunOptions{Transport: StdioTransport})
}

// RunWithOptions starts the MCP server with the requested transport.
func (s *MCPServer) RunWithOptions(ctx context.Context, opts RunOptions) error {
	transport := opts.Transport
	if transport == "" {
		transport = StdioTransport
	}
	if opts.HTTPAddr == "" {
		opts.HTTPAddr = "127.0.0.1:8080"
	}

	switch transport {
	case StdioTransport:
		return s.srv.Run(ctx, &mcp.StdioTransport{})
	case HTTPTransport:
		if !opts.AllowRemote && !config.IsLoopbackListenAddress(opts.HTTPAddr) {
			return fmt.Errorf("refusing non-loopback HTTP listen address %q", opts.HTTPAddr)
		}
		return s.runHTTP(ctx, opts.HTTPAddr)
	default:
		return fmt.Errorf("unsupported transport %q, expected stdio or http", transport)
	}
}

func (s *MCPServer) runHTTP(ctx context.Context, httpAddr string) error {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.srv
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("tree-sitter-mcp: listening on %q", httpAddr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

// GetContainer returns the dependency container for external configuration.
func (s *MCPServer) GetContainer() *container.Container {
	return s.container
}

func readOnlyTool(name, description string) *mcp.Tool {
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}
}

// --- Tool argument types (used for automatic schema inference) ---

type configureArgs struct {
	ConfigPath        *string   `json:"config_path,omitempty"`
	CacheEnabled      *bool     `json:"cache_enabled,omitempty"`
	CacheMaxSizeMB    *int      `json:"cache_max_size_mb,omitempty"`
	CacheTTLSeconds   *int      `json:"cache_ttl_seconds,omitempty"`
	MaxFileSizeMB     *int      `json:"max_file_size_mb,omitempty"`
	AllowedExtensions *[]string `json:"allowed_extensions,omitempty"`
	ExcludedDirs      *[]string `json:"excluded_dirs,omitempty"`
	DefaultMaxDepth   *int      `json:"default_max_depth,omitempty"`
	MaxResultsDefault *int      `json:"max_results_default,omitempty"`
	LogLevel          *string   `json:"log_level,omitempty"`
}

type registerProjectArgs struct {
	Path        string  `json:"path"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type projectNameArgs struct {
	Name string `json:"name"`
}

type languageNameArgs struct {
	Language string `json:"language"`
}

type listFilesArgs struct {
	Project    string   `json:"project"`
	Pattern    *string  `json:"pattern,omitempty"`
	MaxDepth   *int     `json:"max_depth,omitempty"`
	Extensions []string `json:"extensions,omitempty"`
}

type getFileArgs struct {
	Project   string `json:"project"`
	Path      string `json:"path"`
	MaxLines  *int   `json:"max_lines,omitempty"`
	StartLine *int   `json:"start_line,omitempty"`
}

type fileMetadataArgs struct {
	Project string `json:"project"`
	Path    string `json:"path"`
}

type getASTArgs struct {
	Project     string `json:"project"`
	Path        string `json:"path"`
	MaxDepth    *int   `json:"max_depth,omitempty"`
	IncludeText *bool  `json:"include_text,omitempty"`
}

type nodePosArgs struct {
	Project string `json:"project"`
	Path    string `json:"path"`
	Row     int    `json:"row"`
	Column  int    `json:"column"`
}

type parseDiagnosticsArgs struct {
	Project     string `json:"project"`
	Path        string `json:"path"`
	MaxIssues   *int   `json:"max_issues,omitempty"`
	IncludeText *bool  `json:"include_text,omitempty"`
}

type findTextArgs struct {
	Project       string  `json:"project"`
	Pattern       string  `json:"pattern"`
	FilePattern   *string `json:"file_pattern,omitempty"`
	MaxResults    *int    `json:"max_results,omitempty"`
	CaseSensitive *bool   `json:"case_sensitive,omitempty"`
	WholeWord     *bool   `json:"whole_word,omitempty"`
	UseRegex      *bool   `json:"use_regex,omitempty"`
	ContextLines  *int    `json:"context_lines,omitempty"`
}

type runQueryArgs struct {
	Project       string  `json:"project"`
	Query         string  `json:"query"`
	FilePath      *string `json:"file_path,omitempty"`
	Language      *string `json:"language,omitempty"`
	MaxResults    *int    `json:"max_results,omitempty"`
	CaptureFilter *string `json:"capture_filter,omitempty"`
	Compact       *bool   `json:"compact,omitempty"`
}

type queryTemplateArgs struct {
	Language     string `json:"language"`
	TemplateName string `json:"template_name"`
}

type listQueryTemplatesArgs struct {
	Language *string `json:"language,omitempty"`
}

type getSymbolsArgs struct {
	Project     string   `json:"project"`
	FilePath    string   `json:"file_path"`
	SymbolTypes []string `json:"symbol_types,omitempty"`
}

type analyzeProjectArgs struct {
	Project   string `json:"project"`
	ScanDepth *int   `json:"scan_depth,omitempty"`
}

type filePathArgs struct {
	Project  string `json:"project"`
	FilePath string `json:"file_path"`
}

type findUsageArgs struct {
	Project  string  `json:"project"`
	Symbol   string  `json:"symbol"`
	FilePath *string `json:"file_path,omitempty"`
	Language *string `json:"language,omitempty"`
}

type clearCacheArgs struct {
	Project  *string `json:"project,omitempty"`
	FilePath *string `json:"file_path,omitempty"`
}

type buildQueryArgs struct {
	Language string   `json:"language"`
	Patterns []string `json:"patterns"`
	Combine  *string  `json:"combine,omitempty"`
}

type adaptQueryArgs struct {
	Query    string `json:"query"`
	FromLang string `json:"from_language"`
	ToLang   string `json:"to_language"`
}

type getNodeTypesArgs struct {
	Language *string `json:"language,omitempty"`
}

type findSimilarCodeArgs struct {
	Project       string   `json:"project"`
	FilePath      string   `json:"file_path"`
	MaxResults    *int     `json:"max_results,omitempty"`
	MinSimilarity *float64 `json:"min_similarity,omitempty"`
}

type diagnoseConfigArgs struct {
	ConfigPath string `json:"config_path"`
}

func (s *MCPServer) registerTools() {
	// --- configure ---
	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "configure",
		Description: "Configure cache, file security, AST depth, result limits, and logging settings, or load them from a config file.",
	}, s.handleConfigure)

	// --- register_project ---
	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "register_project",
		Description: "Register a project directory for code exploration.",
	}, s.handleRegisterProject)

	// --- list_projects ---
	mcp.AddTool(s.srv, readOnlyTool("list_projects", "List all registered projects."), s.handleListProjects)

	// --- remove_project ---
	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "remove_project",
		Description: "Remove a registered project.",
	}, s.handleRemoveProject)

	// --- list_languages ---
	mcp.AddTool(s.srv, readOnlyTool("list_languages", "List available tree-sitter languages."), s.handleListLanguages)

	// --- check_language ---
	mcp.AddTool(s.srv, readOnlyTool("check_language", "Check if a tree-sitter language parser is available."), s.handleCheckLanguage)

	// --- list_files ---
	mcp.AddTool(s.srv, readOnlyTool("list_files", "List files in a project, optionally filtered by pattern, depth, and extensions."), s.handleListFiles)

	// --- get_file ---
	mcp.AddTool(s.srv, readOnlyTool("get_file", "Get the content of a file in a project."), s.handleGetFile)

	// --- get_file_metadata ---
	mcp.AddTool(s.srv, readOnlyTool("get_file_metadata", "Get metadata for a file (size, modification time, etc.)."), s.handleGetFileMetadata)

	// --- get_ast ---
	mcp.AddTool(s.srv, readOnlyTool("get_ast", "Get the abstract syntax tree (AST) for a file as a nested JSON structure."), s.handleGetAST)

	// --- get_node_at_position ---
	mcp.AddTool(s.srv, readOnlyTool("get_node_at_position", "Find the AST node at a specific row and column position in a file."), s.handleGetNodeAtPosition)

	// --- get_parse_diagnostics ---
	mcp.AddTool(s.srv, readOnlyTool("get_parse_diagnostics", "Report parse health for a file, including ERROR and MISSING syntax nodes."), s.handleGetParseDiagnostics)

	// --- find_text ---
	mcp.AddTool(s.srv, readOnlyTool("find_text", "Search for a text pattern in project files with regex, case, and context support."), s.handleFindText)

	// --- run_query ---
	mcp.AddTool(s.srv, readOnlyTool("run_query", "Run a tree-sitter query (S-expression) on project files."), s.handleRunQuery)

	// --- get_query_template ---
	mcp.AddTool(s.srv, readOnlyTool("get_query_template", "Get a predefined tree-sitter query template (e.g., functions, classes, imports)."), s.handleGetQueryTemplate)

	// --- list_query_templates ---
	mcp.AddTool(s.srv, readOnlyTool("list_query_templates", "List available tree-sitter query templates, optionally filtered by language."), s.handleListQueryTemplates)

	// --- get_symbols ---
	mcp.AddTool(s.srv, readOnlyTool("get_symbols", "Extract symbols (functions, classes, imports, etc.) from a file."), s.handleGetSymbols)

	// --- analyze_project ---
	mcp.AddTool(s.srv, readOnlyTool("analyze_project", "Analyze overall project structure: file counts, languages, top-level files."), s.handleAnalyzeProject)

	// --- get_dependencies ---
	mcp.AddTool(s.srv, readOnlyTool("get_dependencies", "Find the dependencies (imports/includes) of a file."), s.handleGetDependencies)

	// --- analyze_complexity ---
	mcp.AddTool(s.srv, readOnlyTool("analyze_complexity", "Analyze code complexity: line count, function count, average function length."), s.handleAnalyzeComplexity)

	// --- find_usage ---
	mcp.AddTool(s.srv, readOnlyTool("find_usage", "Find all usages of a symbol (identifier) across project files."), s.handleFindUsage)

	// --- clear_cache ---
	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "clear_cache",
		Description: "Clear the parse tree cache, optionally scoped to a project or file.",
	}, s.handleClearCache)

	// --- build_query ---
	mcp.AddTool(s.srv, readOnlyTool("build_query", "Combine query templates or raw patterns into a tree-sitter query union."), s.handleBuildQuery)

	// --- adapt_query ---
	mcp.AddTool(s.srv, readOnlyTool("adapt_query", "Adapt a tree-sitter query from one language to another by translating node type names."), s.handleAdaptQuery)

	// --- get_node_types ---
	mcp.AddTool(s.srv, readOnlyTool("get_node_types", "Get descriptions of common AST node types for a language, or list all available languages."), s.handleGetNodeTypes)

	// --- find_similar_code ---
	mcp.AddTool(s.srv, readOnlyTool("find_similar_code", "Find structurally similar code in a project using AST fingerprinting and Jaccard similarity."), s.handleFindSimilarCode)

	// --- diagnose_config ---
	mcp.AddTool(s.srv, readOnlyTool("diagnose_config", "Diagnose issues with YAML configuration loading (file existence, YAML validity, config changes)."), s.handleDiagnoseConfig)

	// Register prompts.
	s.registerPrompts()
}

// --- Handler Implementations ---

func (s *MCPServer) handleConfigure(ctx context.Context, req *mcp.CallToolRequest, args configureArgs) (*mcp.CallToolResult, any, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	cfgMgr := s.container.ConfigManager
	if err := validateConfigureArgs(args); err != nil {
		return nil, nil, err
	}

	if args.ConfigPath != nil && *args.ConfigPath != "" {
		if err := cfgMgr.LoadFromFile(*args.ConfigPath); err != nil {
			return nil, nil, fmt.Errorf("failed to load config: %w", err)
		}
	}
	if args.CacheEnabled != nil {
		cfgMgr.UpdateValue("cache.enabled", *args.CacheEnabled)
	}
	if args.CacheMaxSizeMB != nil {
		cfgMgr.UpdateValue("cache.max_size_mb", *args.CacheMaxSizeMB)
	}
	if args.CacheTTLSeconds != nil {
		cfgMgr.UpdateValue("cache.ttl_seconds", *args.CacheTTLSeconds)
	}
	if args.MaxFileSizeMB != nil {
		cfgMgr.UpdateValue("security.max_file_size_mb", *args.MaxFileSizeMB)
	}
	if args.AllowedExtensions != nil {
		cfgMgr.UpdateValue("security.allowed_extensions", *args.AllowedExtensions)
	}
	if args.ExcludedDirs != nil {
		cfgMgr.UpdateValue("security.excluded_dirs", *args.ExcludedDirs)
	}
	if args.DefaultMaxDepth != nil {
		cfgMgr.UpdateValue("language.default_max_depth", *args.DefaultMaxDepth)
	}
	if args.MaxResultsDefault != nil {
		cfgMgr.UpdateValue("max_results_default", *args.MaxResultsDefault)
	}
	if args.LogLevel != nil && *args.LogLevel != "" {
		cfgMgr.UpdateValue("log_level", *args.LogLevel)
	}
	s.container.ApplyConfig()

	return textResult(formatJSON(cfgMgr.ToMap())), nil, nil
}

func validateConfigureArgs(args configureArgs) error {
	positive := []struct {
		name  string
		value *int
	}{
		{"cache_max_size_mb", args.CacheMaxSizeMB},
		{"cache_ttl_seconds", args.CacheTTLSeconds},
		{"max_file_size_mb", args.MaxFileSizeMB},
		{"max_results_default", args.MaxResultsDefault},
	}
	for _, field := range positive {
		if field.value != nil && *field.value <= 0 {
			return fmt.Errorf("%s must be greater than zero", field.name)
		}
	}
	if args.DefaultMaxDepth != nil && *args.DefaultMaxDepth < 0 {
		return fmt.Errorf("default_max_depth must not be negative")
	}
	return nil
}

func (s *MCPServer) handleRegisterProject(ctx context.Context, req *mcp.CallToolRequest, args registerProjectArgs) (*mcp.CallToolResult, any, error) {
	name := ""
	if args.Name != nil {
		name = *args.Name
	}
	desc := ""
	if args.Description != nil {
		desc = *args.Description
	}

	cfg := s.container.GetConfig()
	project, err := s.container.ProjectRegistry.RegisterProject(name, args.Path, desc)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to register project: %w", err)
	}

	project.ScanFiles(s.container.LanguageRegistry, cfg.Security.ExcludedDirs)
	return textResult(formatJSON(project.ToMap())), nil, nil
}

func (s *MCPServer) handleListProjects(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	projects := s.container.ProjectRegistry.ListProjects()
	return textResult(formatJSON(projects)), nil, nil
}

func (s *MCPServer) handleRemoveProject(ctx context.Context, req *mcp.CallToolRequest, args projectNameArgs) (*mcp.CallToolResult, any, error) {
	if err := s.container.ProjectRegistry.RemoveProject(args.Name); err != nil {
		return nil, nil, fmt.Errorf("failed to remove project: %w", err)
	}
	return textResult(formatJSON(map[string]string{"status": "success", "message": fmt.Sprintf("Project %q removed", args.Name)})), nil, nil
}

func (s *MCPServer) handleListLanguages(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	langs := s.container.LanguageRegistry.ListAvailableLanguages()
	return textResult(formatJSON(map[string]any{
		"available":   langs,
		"installable": []string{},
	})), nil, nil
}

func (s *MCPServer) handleCheckLanguage(ctx context.Context, req *mcp.CallToolRequest, args languageNameArgs) (*mcp.CallToolResult, any, error) {
	if s.container.LanguageRegistry.IsLanguageAvailable(args.Language) {
		return textResult(formatJSON(map[string]string{"status": "success", "message": fmt.Sprintf("Language %q is available", args.Language)})), nil, nil
	}
	return textResult(formatJSON(map[string]string{"status": "error", "message": fmt.Sprintf("Language %q is not available", args.Language)})), nil, nil
}

func (s *MCPServer) handleListFiles(ctx context.Context, req *mcp.CallToolRequest, args listFilesArgs) (*mcp.CallToolResult, any, error) {
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	pattern := "**/*"
	if args.Pattern != nil {
		pattern = *args.Pattern
	}

	cfg := s.container.GetConfig()
	files, err := tools.ListProjectFiles(project, pattern, args.MaxDepth, args.Extensions, cfg.Security.ExcludedDirs)
	if err != nil {
		return nil, nil, fmt.Errorf("error listing files: %w", err)
	}
	return textResult(formatJSON(files)), nil, nil
}

func (s *MCPServer) handleGetFile(ctx context.Context, req *mcp.CallToolRequest, args getFileArgs) (*mcp.CallToolResult, any, error) {
	if args.StartLine != nil && *args.StartLine < 0 {
		return nil, nil, fmt.Errorf("start_line must not be negative")
	}
	if args.MaxLines != nil && *args.MaxLines <= 0 {
		return nil, nil, fmt.Errorf("max_lines must be greater than zero")
	}
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	startLine := 0
	if args.StartLine != nil {
		startLine = *args.StartLine
	}

	content, err := tools.GetFileContent(project, args.Path, args.MaxLines, startLine)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading file: %w", err)
	}
	return textResult(content), nil, nil
}

func (s *MCPServer) handleGetFileMetadata(ctx context.Context, req *mcp.CallToolRequest, args fileMetadataArgs) (*mcp.CallToolResult, any, error) {
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	info, err := tools.GetFileInfo(project, args.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting file info: %w", err)
	}
	return textResult(formatJSON(info)), nil, nil
}

func (s *MCPServer) handleGetAST(ctx context.Context, req *mcp.CallToolRequest, args getASTArgs) (*mcp.CallToolResult, any, error) {
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	cfg := s.container.GetConfig()
	maxDepth := cfg.Language.DefaultMaxDepth
	if args.MaxDepth != nil {
		maxDepth = *args.MaxDepth
	}
	if maxDepth < 0 {
		return nil, nil, fmt.Errorf("max_depth must not be negative")
	}

	includeText := true
	if args.IncludeText != nil {
		includeText = *args.IncludeText
	}

	ast, err := tools.GetFileAST(project, args.Path, s.container.LanguageRegistry, s.container.TreeCache, &maxDepth, includeText)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing AST: %w", err)
	}
	return textResult(formatJSON(ast)), nil, nil
}

func (s *MCPServer) handleGetNodeAtPosition(ctx context.Context, req *mcp.CallToolRequest, args nodePosArgs) (*mcp.CallToolResult, any, error) {
	if args.Row < 0 || args.Column < 0 {
		return nil, nil, fmt.Errorf("row and column must not be negative")
	}
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	absPath, err := project.ResolveFilePath(args.Path)
	if err != nil {
		return nil, nil, err
	}
	lang := s.container.LanguageRegistry.LanguageForFile(args.Path)
	if lang == "" {
		return nil, nil, fmt.Errorf("could not detect language for %s", args.Path)
	}

	tree, sourceBytes, err := tools.ParseFile(absPath, lang, s.container.LanguageRegistry, s.container.TreeCache)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing file: %w", err)
	}
	defer tree.Close()

	node := tools.FindNodeAtPos(tree.RootNode(), args.Row, args.Column)
	if node == nil {
		return textResult("null"), nil, nil
	}

	result := models.NodeToMap(node, sourceBytes, true, true, 2)
	return textResult(formatJSON(result)), nil, nil
}

func (s *MCPServer) handleGetParseDiagnostics(ctx context.Context, req *mcp.CallToolRequest, args parseDiagnosticsArgs) (*mcp.CallToolResult, any, error) {
	if args.MaxIssues != nil && *args.MaxIssues <= 0 {
		return nil, nil, fmt.Errorf("max_issues must be greater than zero")
	}
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	maxIssues := 100
	if args.MaxIssues != nil {
		maxIssues = *args.MaxIssues
	}
	includeText := false
	if args.IncludeText != nil {
		includeText = *args.IncludeText
	}

	result, err := tools.GetParseDiagnostics(project, args.Path, s.container.LanguageRegistry, s.container.TreeCache, maxIssues, includeText)
	if err != nil {
		return nil, nil, fmt.Errorf("parse diagnostics error: %w", err)
	}

	return textResult(formatJSON(result)), nil, nil
}

func (s *MCPServer) handleFindText(ctx context.Context, req *mcp.CallToolRequest, args findTextArgs) (*mcp.CallToolResult, any, error) {
	if args.MaxResults != nil && *args.MaxResults <= 0 {
		return nil, nil, fmt.Errorf("max_results must be greater than zero")
	}
	if args.ContextLines != nil && *args.ContextLines < 0 {
		return nil, nil, fmt.Errorf("context_lines must not be negative")
	}
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	maxResults := s.container.GetConfig().MaxResultsDefault
	if args.MaxResults != nil {
		maxResults = *args.MaxResults
	}

	caseSensitive := false
	if args.CaseSensitive != nil {
		caseSensitive = *args.CaseSensitive
	}
	wholeWord := false
	if args.WholeWord != nil {
		wholeWord = *args.WholeWord
	}
	useRegex := false
	if args.UseRegex != nil {
		useRegex = *args.UseRegex
	}
	contextLines := 2
	if args.ContextLines != nil {
		contextLines = *args.ContextLines
	}

	filePattern := "**/*"
	if args.FilePattern != nil {
		filePattern = *args.FilePattern
	}

	cfg := s.container.GetConfig()
	results, err := tools.SearchText(project, args.Pattern, filePattern, maxResults, caseSensitive, wholeWord, useRegex, contextLines, cfg.Security.ExcludedDirs)
	if err != nil {
		return nil, nil, fmt.Errorf("search error: %w", err)
	}
	return textResult(formatJSON(results)), nil, nil
}

func (s *MCPServer) handleRunQuery(ctx context.Context, req *mcp.CallToolRequest, args runQueryArgs) (*mcp.CallToolResult, any, error) {
	if args.MaxResults != nil && *args.MaxResults <= 0 {
		return nil, nil, fmt.Errorf("max_results must be greater than zero")
	}
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	maxResults := s.container.GetConfig().MaxResultsDefault
	if args.MaxResults != nil {
		maxResults = *args.MaxResults
	}

	filePath := ""
	if args.FilePath != nil {
		filePath = *args.FilePath
	}
	lang := ""
	if args.Language != nil {
		lang = *args.Language
	}
	captureFilter := ""
	if args.CaptureFilter != nil {
		captureFilter = *args.CaptureFilter
	}
	compact := false
	if args.Compact != nil {
		compact = *args.Compact
	}

	cfg := s.container.GetConfig()
	results, err := tools.RunQuery(project, args.Query, s.container.LanguageRegistry, s.container.TreeCache, filePath, lang, maxResults, captureFilter, compact, cfg.Security.ExcludedDirs)
	if err != nil {
		return nil, nil, fmt.Errorf("query error: %w", err)
	}
	return textResult(formatJSON(results)), nil, nil
}

func (s *MCPServer) handleGetQueryTemplate(ctx context.Context, req *mcp.CallToolRequest, args queryTemplateArgs) (*mcp.CallToolResult, any, error) {
	tmpl := language.GetQueryTemplate(args.Language, args.TemplateName)
	if tmpl == "" {
		return nil, nil, fmt.Errorf("no template '%s' for language '%s'", args.TemplateName, args.Language)
	}

	return textResult(formatJSON(map[string]string{
		"language": args.Language,
		"name":     args.TemplateName,
		"query":    tmpl,
	})), nil, nil
}

func (s *MCPServer) handleListQueryTemplates(ctx context.Context, req *mcp.CallToolRequest, args listQueryTemplatesArgs) (*mcp.CallToolResult, any, error) {
	lang := ""
	if args.Language != nil {
		lang = *args.Language
	}
	result := language.ListQueryTemplates(lang)
	return textResult(formatJSON(result)), nil, nil
}

func (s *MCPServer) handleGetSymbols(ctx context.Context, req *mcp.CallToolRequest, args getSymbolsArgs) (*mcp.CallToolResult, any, error) {
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	symbols, err := tools.ExtractSymbols(project, args.FilePath, s.container.LanguageRegistry, s.container.TreeCache, args.SymbolTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("symbol extraction error: %w", err)
	}
	return textResult(formatJSON(symbols)), nil, nil
}

func (s *MCPServer) handleAnalyzeProject(ctx context.Context, req *mcp.CallToolRequest, args analyzeProjectArgs) (*mcp.CallToolResult, any, error) {
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	scanDepth := 3
	if args.ScanDepth != nil {
		scanDepth = *args.ScanDepth
	}

	cfg := s.container.GetConfig()
	analysis, err := tools.AnalyzeProjectStructure(project, s.container.LanguageRegistry, scanDepth, cfg.Security.ExcludedDirs)
	if err != nil {
		return nil, nil, fmt.Errorf("analysis error: %w", err)
	}
	return textResult(formatJSON(analysis)), nil, nil
}

func (s *MCPServer) handleGetDependencies(ctx context.Context, req *mcp.CallToolRequest, args filePathArgs) (*mcp.CallToolResult, any, error) {
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	deps, err := tools.FindDependencies(project, args.FilePath, s.container.LanguageRegistry, s.container.TreeCache)
	if err != nil {
		return nil, nil, fmt.Errorf("dependency analysis error: %w", err)
	}
	return textResult(formatJSON(deps)), nil, nil
}

func (s *MCPServer) handleAnalyzeComplexity(ctx context.Context, req *mcp.CallToolRequest, args filePathArgs) (*mcp.CallToolResult, any, error) {
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	info, err := tools.AnalyzeComplexity(project, args.FilePath, s.container.LanguageRegistry, s.container.TreeCache)
	if err != nil {
		return nil, nil, fmt.Errorf("complexity analysis error: %w", err)
	}
	return textResult(formatJSON(info)), nil, nil
}

func (s *MCPServer) handleFindUsage(ctx context.Context, req *mcp.CallToolRequest, args findUsageArgs) (*mcp.CallToolResult, any, error) {
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	lang := ""
	if args.Language != nil {
		lang = *args.Language
	}
	filePath := ""
	if args.FilePath != nil {
		filePath = *args.FilePath
	}

	if lang == "" && filePath != "" {
		lang = s.container.LanguageRegistry.LanguageForFile(filePath)
	}
	if lang == "" {
		return nil, nil, fmt.Errorf("either language or file_path must be provided")
	}

	query := fmt.Sprintf(`((identifier) @reference (#eq? @reference %s))`, strconv.Quote(args.Symbol))
	cfg := s.container.GetConfig()
	results, err := tools.RunQuery(project, query, s.container.LanguageRegistry, s.container.TreeCache, filePath, lang, cfg.MaxResultsDefault, "", false, cfg.Security.ExcludedDirs)
	if err != nil {
		return nil, nil, fmt.Errorf("usage search error: %w", err)
	}
	return textResult(formatJSON(results)), nil, nil
}

func (s *MCPServer) handleBuildQuery(ctx context.Context, req *mcp.CallToolRequest, args buildQueryArgs) (*mcp.CallToolResult, any, error) {
	combine := "or"
	if args.Combine != nil {
		combine = *args.Combine
	}

	result, err := tools.BuildQuery(args.Language, args.Patterns, combine)
	if err != nil {
		return nil, nil, fmt.Errorf("build_query error: %w", err)
	}
	return textResult(formatJSON(result)), nil, nil
}

func (s *MCPServer) handleAdaptQuery(ctx context.Context, req *mcp.CallToolRequest, args adaptQueryArgs) (*mcp.CallToolResult, any, error) {
	result, err := tools.AdaptQuery(args.Query, args.FromLang, args.ToLang)
	if err != nil {
		return nil, nil, fmt.Errorf("adapt_query error: %w", err)
	}
	return textResult(formatJSON(result)), nil, nil
}

func (s *MCPServer) handleGetNodeTypes(ctx context.Context, req *mcp.CallToolRequest, args getNodeTypesArgs) (*mcp.CallToolResult, any, error) {
	if args.Language != nil && *args.Language != "" {
		result, err := tools.GetNodeTypes(*args.Language)
		if err != nil {
			return nil, nil, fmt.Errorf("get_node_types error: %w", err)
		}
		return textResult(formatJSON(result)), nil, nil
	}
	// List all available languages.
	result := tools.ListAllNodeTypes()
	return textResult(formatJSON(result)), nil, nil
}

func (s *MCPServer) handleFindSimilarCode(ctx context.Context, req *mcp.CallToolRequest, args findSimilarCodeArgs) (*mcp.CallToolResult, any, error) {
	if args.MaxResults != nil && *args.MaxResults <= 0 {
		return nil, nil, fmt.Errorf("max_results must be greater than zero")
	}
	if args.MinSimilarity != nil && (*args.MinSimilarity < 0 || *args.MinSimilarity > 1) {
		return nil, nil, fmt.Errorf("min_similarity must be between 0 and 1")
	}
	project, err := s.container.ProjectRegistry.GetProject(args.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("project error: %w", err)
	}

	maxResults := 10
	if args.MaxResults != nil {
		maxResults = *args.MaxResults
	}
	minSimilarity := 0.5
	if args.MinSimilarity != nil {
		minSimilarity = *args.MinSimilarity
	}

	cfg := s.container.GetConfig()
	results, err := tools.FindSimilarCode(project, args.FilePath, s.container.LanguageRegistry, s.container.TreeCache, maxResults, minSimilarity, cfg.Security.ExcludedDirs)
	if err != nil {
		return nil, nil, fmt.Errorf("find_similar_code error: %w", err)
	}
	return textResult(formatJSON(results)), nil, nil
}

func (s *MCPServer) handleDiagnoseConfig(ctx context.Context, req *mcp.CallToolRequest, args diagnoseConfigArgs) (*mcp.CallToolResult, any, error) {
	result := tools.DiagnoseYamlConfig(args.ConfigPath, s.container.ConfigManager)
	return textResult(formatJSON(result)), nil, nil
}

func (s *MCPServer) handleClearCache(ctx context.Context, req *mcp.CallToolRequest, args clearCacheArgs) (*mcp.CallToolResult, any, error) {
	projName := ""
	if args.Project != nil {
		projName = *args.Project
	}
	filePath := ""
	if args.FilePath != nil {
		filePath = *args.FilePath
	}

	if filePath != "" && projName == "" {
		return nil, nil, fmt.Errorf("project is required when file_path is provided")
	}
	if projName != "" {
		project, err := s.container.ProjectRegistry.GetProject(projName)
		if err != nil {
			return nil, nil, fmt.Errorf("project error: %w", err)
		}
		target := project.RootPath
		message := fmt.Sprintf("Cache cleared for project %s", projName)
		if filePath != "" {
			target, err = project.ResolveFilePath(filePath)
			if err != nil {
				return nil, nil, err
			}
			message = fmt.Sprintf("Cache cleared for %s in %s", filePath, projName)
		}
		s.container.TreeCache.Invalidate(target)
		return textResult(formatJSON(map[string]string{"status": "success", "message": message})), nil, nil
	}

	s.container.TreeCache.Invalidate()
	return textResult(formatJSON(map[string]string{"status": "success", "message": "All caches cleared"})), nil, nil
}

// --- Prompts ---

func (s *MCPServer) registerPrompts() {
	// code_review prompt
	s.srv.AddPrompt(&mcp.Prompt{
		Name:        "code_review",
		Description: "Generate a code review prompt for a file, including structural information.",
		Arguments: []*mcp.PromptArgument{
			{Name: "project", Description: "Project name", Required: true},
			{Name: "file_path", Description: "Path to the file to review", Required: true},
		},
	}, s.handleCodeReviewPrompt)

	// explain_code prompt
	s.srv.AddPrompt(&mcp.Prompt{
		Name:        "explain_code",
		Description: "Generate a prompt to explain a code file's functionality and structure.",
		Arguments: []*mcp.PromptArgument{
			{Name: "project", Description: "Project name", Required: true},
			{Name: "file_path", Description: "Path to the file to explain", Required: true},
			{Name: "focus", Description: "Optional aspect to focus the explanation on", Required: false},
		},
	}, s.handleExplainCodePrompt)

	// explain_tree_sitter_query prompt
	s.srv.AddPrompt(&mcp.Prompt{
		Name:        "explain_tree_sitter_query",
		Description: "Generate a prompt explaining tree-sitter query syntax with examples.",
	}, s.handleExplainQueryPrompt)

	// suggest_improvements prompt
	s.srv.AddPrompt(&mcp.Prompt{
		Name:        "suggest_improvements",
		Description: "Generate a prompt suggesting code improvements based on complexity metrics.",
		Arguments: []*mcp.PromptArgument{
			{Name: "project", Description: "Project name", Required: true},
			{Name: "file_path", Description: "Path to the file to analyze", Required: true},
		},
	}, s.handleSuggestImprovementsPrompt)

	// project_overview prompt
	s.srv.AddPrompt(&mcp.Prompt{
		Name:        "project_overview",
		Description: "Generate a prompt for analyzing and summarizing a project's structure.",
		Arguments: []*mcp.PromptArgument{
			{Name: "project", Description: "Project name", Required: true},
		},
	}, s.handleProjectOverviewPrompt)
}

func (s *MCPServer) handleCodeReviewPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectName := req.Params.Arguments["project"]
	filePath := req.Params.Arguments["file_path"]

	project, err := s.container.ProjectRegistry.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("project error: %w", err)
	}

	content, err := tools.GetFileContent(project, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("file read error: %w", err)
	}

	lang := s.container.LanguageRegistry.LanguageForFile(filePath)

	// Get structure info.
	structure := ""
	symbols, symErr := tools.ExtractSymbols(project, filePath, s.container.LanguageRegistry, s.container.TreeCache, nil)
	if symErr == nil {
		if funcs, ok := symbols["functions"]; ok && len(funcs) > 0 {
			structure += "\nFunctions:\n"
			for _, f := range funcs {
				structure += fmt.Sprintf("- %s\n", f.Name)
			}
		}
		if classes, ok := symbols["classes"]; ok && len(classes) > 0 {
			structure += "\nClasses:\n"
			for _, c := range classes {
				structure += fmt.Sprintf("- %s\n", c.Name)
			}
		}
	}

	promptText := fmt.Sprintf(`Please review this %s code file:

`+"```"+`%s
%s
`+"```"+`

%s

Focus on:
1. Code clarity and organization
2. Potential bugs or issues
3. Performance considerations
4. Best practices for %s`, lang, lang, content, structure, lang)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Code review for %s", filePath),
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: promptText}},
		},
	}, nil
}

func (s *MCPServer) handleExplainCodePrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectName := req.Params.Arguments["project"]
	filePath := req.Params.Arguments["file_path"]
	focus := req.Params.Arguments["focus"]

	project, err := s.container.ProjectRegistry.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("project error: %w", err)
	}

	content, err := tools.GetFileContent(project, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("file read error: %w", err)
	}

	lang := s.container.LanguageRegistry.LanguageForFile(filePath)

	focusPrompt := ""
	if focus != "" {
		focusPrompt = fmt.Sprintf("\nPlease focus specifically on explaining: %s", focus)
	}

	promptText := fmt.Sprintf(`Please explain this %s code file:

`+"```"+`%s
%s
`+"```"+`

Provide a clear explanation of:
1. What this code does
2. How it's structured
3. Any important patterns or techniques used
%s`, lang, lang, content, focusPrompt)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Explanation for %s", filePath),
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: promptText}},
		},
	}, nil
}

func (s *MCPServer) handleExplainQueryPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	promptText := `Tree-sitter queries use S-expression syntax to match patterns in code.

Basic query syntax:
- (node_type) - Match nodes of a specific type
- (node_type field: (child_type)) - Match nodes with specific field relationships
- @name - Capture a node with a name
- #predicate - Apply additional constraints

Example query for Python functions:
` + "```" + `
(function_definition
  name: (identifier) @function.name
  parameters: (parameters) @function.params
  body: (block) @function.body) @function.def
` + "```" + `

Please write a tree-sitter query to find:`

	return &mcp.GetPromptResult{
		Description: "Tree-sitter query syntax explanation",
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: promptText}},
		},
	}, nil
}

func (s *MCPServer) handleSuggestImprovementsPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectName := req.Params.Arguments["project"]
	filePath := req.Params.Arguments["file_path"]

	project, err := s.container.ProjectRegistry.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("project error: %w", err)
	}

	content, err := tools.GetFileContent(project, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("file read error: %w", err)
	}

	lang := s.container.LanguageRegistry.LanguageForFile(filePath)

	complexityInfo := ""
	info, compErr := tools.AnalyzeComplexity(project, filePath, s.container.LanguageRegistry, s.container.TreeCache)
	if compErr == nil {
		complexityInfo = fmt.Sprintf(`
Code metrics:
- Line count: %d
- Functions: %d
- Avg. function length: %d lines
`, info.TotalLines, info.FunctionCount, info.AvgLength)
	}

	promptText := fmt.Sprintf(`Please suggest improvements for this %s code:

`+"```"+`%s
%s
`+"```"+`

%s
Suggest specific, actionable improvements for:
1. Code quality and readability
2. Performance optimization
3. Error handling and robustness
4. Following %s best practices

Where possible, provide code examples of your suggestions.`, lang, lang, content, complexityInfo, lang)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Improvement suggestions for %s", filePath),
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: promptText}},
		},
	}, nil
}

func (s *MCPServer) handleProjectOverviewPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectName := req.Params.Arguments["project"]

	project, err := s.container.ProjectRegistry.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("project error: %w", err)
	}

	cfg := s.container.GetConfig()
	analysis, err := tools.AnalyzeProjectStructure(project, s.container.LanguageRegistry, 3, cfg.Security.ExcludedDirs)
	if err != nil {
		return nil, fmt.Errorf("analysis error: %w", err)
	}

	languagesStr := ""
	for lang, count := range analysis.Languages {
		languagesStr += fmt.Sprintf("- %s: %d files\n", lang, count)
	}

	topFilesStr := ""
	for _, f := range analysis.TopFiles {
		topFilesStr += fmt.Sprintf("- %s\n", f)
	}
	if topFilesStr == "" {
		topFilesStr = "None detected"
	}

	promptText := fmt.Sprintf(`Please analyze this codebase:

Project name: %s
Path: %s

Languages:
%s

Top-level files:
%s

Please provide:
1. An overview of the project structure
2. The likely purpose and architecture
3. Key entry points and build configuration
4. Any notable patterns or conventions`, project.Name, project.RootPath, languagesStr, topFilesStr)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Project overview for %s", projectName),
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: promptText}},
		},
	}, nil
}

// --- Helpers ---

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func formatJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return `{"error":` + strconv.Quote(err.Error()) + `}`
	}
	return string(data)
}
