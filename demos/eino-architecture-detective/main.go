package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	mcptool "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const projectName = "detective-target"

func main() {
	log.SetFlags(0)

	envFile := flag.String("env", ".env", "path to the .env file containing LLM credentials")
	flag.Parse()

	if err := loadDotEnv(*envFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load .env: %v", err)
	}

	targetPath := "."
	if flag.NArg() > 0 {
		targetPath = flag.Arg(0)
	}
	focus := "Find the architectural center of gravity, surprising implementation choices, and the best next refactor."
	if flag.NArg() > 1 {
		focus = flag.Arg(1)
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		log.Fatalf("resolve target path: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	model, err := newChatModel(ctx)
	if err != nil {
		log.Fatalf("new chat model: %v", err)
	}

	printMCPConnection(absTarget)

	mcpTools, closeMCP, err := connectMCPTools(ctx)
	if err != nil {
		log.Fatalf("connect MCP tools: %v", err)
	}
	defer closeMCP()

	evidence, err := gatherEvidence(ctx, mcpTools, absTarget)
	if err != nil {
		log.Fatalf("gather evidence: %v", err)
	}

	report, err := model.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You are a senior software architect. Use only the supplied tree-sitter MCP evidence. Be specific, concise, and cite file names when possible."),
		schema.UserMessage(buildPrompt(absTarget, focus, evidence)),
	})
	if err != nil {
		log.Fatalf("generate report: %v", err)
	}

	fmt.Println(report.Content)
}

func connectMCPTools(ctx context.Context) ([]tool.BaseTool, func(), error) {
	var cli *client.Client
	var err error
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MCP_TS_TRANSPORT"))) {
	case "", "stdio":
		command, args := serverCommand()
		cli, err = client.NewStdioMCPClient(command, os.Environ(), args...)
	case "sse":
		cli, err = client.NewSSEMCPClient(sseEndpoint())
		if err == nil {
			err = cli.Start(ctx)
		}
	default:
		err = fmt.Errorf("unsupported MCP_TS_TRANSPORT %q, expected stdio or sse", os.Getenv("MCP_TS_TRANSPORT"))
	}
	if err != nil {
		return nil, nil, err
	}
	closeClient := func() {
		if err := cli.Close(); err != nil {
			log.Printf("close MCP client: %v", err)
		}
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "eino-architecture-detective",
		Version: "0.1.0",
	}
	if _, err := cli.Initialize(ctx, initRequest); err != nil {
		closeClient()
		return nil, nil, err
	}

	tools, err := mcptool.GetTools(ctx, &mcptool.Config{Cli: cli})
	if err != nil {
		closeClient()
		return nil, nil, err
	}

	return tools, closeClient, nil
}

func printMCPConnection(target string) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MCP_TS_TRANSPORT"))) {
	case "sse":
		fmt.Printf("Connecting to tree-sitter MCP server over SSE at %q and inspecting %s\n\n", sseEndpoint(), target)
	default:
		fmt.Printf("Connecting to tree-sitter MCP server over stdio and inspecting %s\n\n", target)
	}
}

func sseEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("MCP_TS_SSE_URL")); endpoint != "" {
		return endpoint
	}
	addr := strings.TrimSpace(os.Getenv("MCP_TS_HTTP_ADDR"))
	if addr == "" {
		addr = ":8080"
	}
	path := strings.TrimSpace(os.Getenv("MCP_TS_SSE_PATH"))
	if path == "" {
		path = "/sse"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + strings.TrimRight(addr, "/") + path
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + path
}

func serverCommand() (string, []string) {
	if bin := strings.TrimSpace(os.Getenv("TREE_SITTER_MCP_BIN")); bin != "" {
		return bin, nil
	}
	return "go", []string{"run", "."}
}

func newChatModel(ctx context.Context) (*openai.ChatModel, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is missing; add it to .env")
	}

	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4.1-mini"
	}

	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    os.Getenv("OPENAI_BASE_URL"),
		ByAzure:    strings.EqualFold(os.Getenv("OPENAI_BY_AZURE"), "true"),
		APIVersion: os.Getenv("OPENAI_API_VERSION"),
	})
}

type evidenceItem struct {
	Tool   string
	Args   string
	Result string
}

func gatherEvidence(ctx context.Context, tools []tool.BaseTool, target string) ([]evidenceItem, error) {
	var evidence []evidenceItem
	call := func(name string, args map[string]any) (string, error) {
		result, err := callTool(ctx, tools, name, args)
		if err != nil {
			return "", err
		}
		evidence = append(evidence, evidenceItem{
			Tool:   name,
			Args:   mustJSON(args),
			Result: truncate(result, 9000),
		})
		fmt.Printf("MCP %-20s ok\n", name)
		return result, nil
	}

	if _, err := call("register_project", map[string]any{
		"path":        target,
		"name":        projectName,
		"description": "Project being inspected by the Eino architecture detective demo.",
	}); err != nil {
		return nil, err
	}

	if _, err := call("list_languages", map[string]any{}); err != nil {
		return nil, err
	}

	if _, err := call("analyze_project", map[string]any{
		"project":    projectName,
		"scan_depth": 4,
	}); err != nil {
		return nil, err
	}

	fileResult, err := call("list_files", map[string]any{
		"project":    projectName,
		"pattern":    "**/*",
		"max_depth":  3,
		"extensions": []string{"go", "py", "js", "ts", "rs"},
	})
	if err != nil {
		return nil, err
	}

	files := interestingFiles(fileResult, 5)
	for _, file := range files {
		_, _ = call("get_symbols", map[string]any{
			"project":   projectName,
			"file_path": file,
		})
		_, _ = call("get_dependencies", map[string]any{
			"project":   projectName,
			"file_path": file,
		})
		_, _ = call("analyze_complexity", map[string]any{
			"project":   projectName,
			"file_path": file,
		})
	}

	if len(files) > 0 {
		_, _ = call("get_ast", map[string]any{
			"project":      projectName,
			"path":         files[0],
			"max_depth":    2,
			"include_text": false,
		})
		_, _ = call("find_similar_code", map[string]any{
			"project":        projectName,
			"file_path":      files[0],
			"max_results":    5,
			"min_similarity": 0.25,
		})
	}

	if goFile := firstWithExt(files, ".go"); goFile != "" {
		_, _ = call("run_query", map[string]any{
			"project":     projectName,
			"file_path":   goFile,
			"language":    "go",
			"max_results": 25,
			"compact":     true,
			"query":       "(function_declaration name: (identifier) @function.name) @function.def",
		})
	}

	return evidence, nil
}

func callTool(ctx context.Context, tools []tool.BaseTool, name string, args map[string]any) (string, error) {
	for _, candidate := range tools {
		info, err := candidate.Info(ctx)
		if err != nil {
			return "", err
		}
		if info.Name != name {
			continue
		}
		invokable, ok := candidate.(tool.InvokableTool)
		if !ok {
			return "", fmt.Errorf("tool %s is not invokable", name)
		}
		return invokable.InvokableRun(ctx, mustJSON(args))
	}
	return "", fmt.Errorf("MCP tool not found: %s", name)
}

func interestingFiles(raw string, limit int) []string {
	var files []string
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return nil
	}

	weights := map[string]int{
		"main.go":   0,
		"server.go": 1,
		"go.mod":    2,
		".go":       3,
		".rs":       4,
		".py":       5,
		".ts":       6,
		".js":       7,
	}

	sort.SliceStable(files, func(i, j int) bool {
		return fileScore(files[i], weights) < fileScore(files[j], weights)
	})

	if len(files) > limit {
		files = files[:limit]
	}
	return files
}

func fileScore(file string, weights map[string]int) int {
	base := filepath.Base(file)
	if score, ok := weights[base]; ok {
		return score
	}
	if strings.Contains(file, "internal/server") {
		return 2
	}
	ext := filepath.Ext(file)
	if score, ok := weights[ext]; ok {
		return score + strings.Count(file, "/")
	}
	return 100 + strings.Count(file, "/")
}

func firstWithExt(files []string, ext string) string {
	for _, file := range files {
		if filepath.Ext(file) == ext {
			return file
		}
	}
	return ""
}

func buildPrompt(target, focus string, evidence []evidenceItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Target project: %s\n", target)
	fmt.Fprintf(&b, "Focus question: %s\n\n", focus)
	b.WriteString("Write an architecture detective report with these sections:\n")
	b.WriteString("1. One-paragraph executive read\n")
	b.WriteString("2. Evidence-backed map of important modules\n")
	b.WriteString("3. Three surprising or non-obvious findings\n")
	b.WriteString("4. Best next refactor or investigation\n\n")
	b.WriteString("Tree-sitter MCP evidence follows. Treat it as the source of truth.\n\n")
	for i, item := range evidence {
		fmt.Fprintf(&b, "### Evidence %d: %s\nargs: %s\nresult:\n%s\n\n", i+1, item.Tool, item.Args, item.Result)
	}
	return b.String()
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... truncated ..."
}
