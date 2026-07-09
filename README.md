# tree-sitter-mcp

An [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) server that gives AI assistants structured access to codebases via [tree-sitter](https://tree-sitter.github.io/). It can parse ASTs, extract symbols, run S-expression queries, find similar code, and analyze project structure through a standardized MCP interface.

## Features

- **20+ MCP tools** covering file ops, AST inspection, symbol extraction, text/regex search, tree-sitter queries, complexity analysis, and more
- **stdio and streamable HTTP transports** using the same MCP transport shape as the companion MCP servers
- **Bundled parsers** for C, C++, Go, HTML, Java, JavaScript, JSON, PHP, Python, Ruby, and Rust
- **Extension detection** for additional file types in project summaries and file filtering
- **Project registry** to register multiple project directories and scope operations to each
- **AST as JSON** with full or depth-limited abstract syntax trees
- **Symbol extraction** using built-in query templates
- **S-expression queries** with direct tree-sitter query execution
- **Query builder** for combining query templates and adapting queries across languages
- **Structural search** using AST fingerprinting and Jaccard similarity
- **Parse tree caching** with configurable in-memory cache size and TTL
- **Pre-parsing** via `--pre-parse` to warm the parse cache at startup
- **YAML configuration** for cache size, file security limits, excluded dirs, and more
- **Diagnostics** with `diagnose_config` for troubleshooting YAML config loading

## Installation

Build from source with Go and Task:

```sh
task build
```

The binary is written to `bin/tree-sitter-mcp` or `bin/tree-sitter-mcp.exe` on Windows. The build requires CGO to compile the bundled tree-sitter parsers, so ensure you have a C compiler available on `PATH` or set `CC`/`CXX` before running build and test commands.

## Usage

### Command-line Flags

```text
tree-sitter-mcp [flags]

Flags:
  --config string       Path to YAML configuration file
  --debug               Enable debug logging
  --disable-cache       Disable parse tree caching
  --pre-parse string    Pre-parse all source files in a directory at startup
  --transport string    MCP transport: stdio or http (default "stdio")
  --http-addr string    HTTP listen address when using --transport=http (default ":8080")
  --version             Show version and exit
```

### Running as an MCP Server

By default, the server communicates over stdio. Configure your MCP client to launch it:

```json
{
  "mcpServers": {
    "tree-sitter": {
      "command": "/path/to/bin/tree-sitter-mcp",
      "args": ["--config", "/path/to/config.yaml"]
    }
  }
}
```

To serve MCP over streamable HTTP instead, run the server as an HTTP process:

```sh
tree-sitter-mcp --transport=http --http-addr=:8080
```

Then configure a streamable HTTP-capable MCP client to connect to:

```text
http://localhost:8080
```

The same settings can be provided with `MCP_TRANSPORT` and `MCP_HTTP_ADDR`.

### Pre-parsing a Project

Use `--pre-parse` to walk a directory and parse source files with bundled parsers into the cache before the MCP server starts accepting requests. This eliminates first-query latency for subsequent `get_ast`, `run_query`, and `get_symbols` calls.

```sh
tree-sitter-mcp --debug --pre-parse /path/to/project
```

Hidden files/directories (`.git`, `.vscode`, etc.) and configured `excluded_dirs` are skipped. The server logs a summary after pre-parsing completes:

```text
Pre-parsing project at /path/to/project ...
Pre-parse complete: 142 files scanned, 130 parsed, 10 skipped, 2 errors in 2.3s
  go: 85 files
  python: 30 files
Starting tree-sitter MCP server (cache: true, max_file_size: 5MB, max_depth: 5)
```

## Configuration

Create a YAML config file. Defaults are used when no config file is supplied.

```yaml
cache:
  enabled: true
  max_size_mb: 100
  ttl_seconds: 300

security:
  max_file_size_mb: 5
  allowed_extensions: []
  excluded_dirs:
    - .git
    - node_modules
    - __pycache__
    - .venv
    - venv
    - .tox

language:
  default_max_depth: 5
  preferred_languages:
    - go
    - python

log_level: INFO
max_results_default: 100
```

Flags override environment variables.

Environment variable overrides currently supported:

- `TREE_SITTER_MCP_LOG_LEVEL`
- `TREE_SITTER_MCP_CACHE_MAX_SIZE_MB`
- `MCP_TRANSPORT` (`stdio` or `http`)
- `MCP_HTTP_ADDR`

## MCP Tools

### Project Management

| Tool | Description |
|------|-------------|
| `register_project` | Register a project directory for code exploration |
| `list_projects` | List all registered projects |
| `remove_project` | Remove a registered project |
| `analyze_project` | Analyze project structure: file counts, languages, top-level files |

### File Operations

| Tool | Description |
|------|-------------|
| `list_files` | List files in a project, filtered by basename/path glob, depth, and extensions |
| `get_file` | Get file content with optional line range limits |
| `get_file_metadata` | Get file metadata (size, modification time, language) |

### AST & Parsing

| Tool | Description |
|------|-------------|
| `get_ast` | Get the full AST for a file as nested JSON |
| `get_node_at_position` | Find the AST node at a specific row/column |
| `list_languages` | List available tree-sitter languages |
| `check_language` | Check if a language parser is available |

### Symbols

| Tool | Description |
|------|-------------|
| `get_symbols` | Extract symbols from a file |
| `find_usage` | Find usages of a symbol/identifier across project files |
| `get_dependencies` | Find the dependencies/imports/includes of a file |

### Search

| Tool | Description |
|------|-------------|
| `find_text` | Search for text/regex in project files with file-pattern and context-line support |
| `find_similar_code` | Find structurally similar code using AST fingerprinting |
| `run_query` | Run a raw tree-sitter S-expression query on project files |

### Queries

| Tool | Description |
|------|-------------|
| `get_query_template` | Get a predefined tree-sitter query template |
| `list_query_templates` | List available tree-sitter query templates |
| `build_query` | Combine multiple templates/patterns into a compound query |
| `adapt_query` | Adapt a query from one language to another by translating node types |
| `get_node_types` | Get descriptions of common AST node types for a language |

### Analysis

| Tool | Description |
|------|-------------|
| `analyze_complexity` | Analyze line count, function count, and average function length |

### Utilities

| Tool | Description |
|------|-------------|
| `configure` | Dynamically reconfigure server settings at runtime |
| `clear_cache` | Clear the parse tree cache, optionally scoped to project/file |
| `diagnose_config` | Diagnose YAML configuration loading issues |

## Language Support

Bundled tree-sitter parsers are available for AST, query, symbol, dependency, complexity, and similarity operations:

| Language | Extensions |
|----------|-----------|
| C | `.c`, `.h` |
| C++ | `.cpp`, `.cc`, `.hpp` |
| Go | `.go` |
| HTML | `.html` |
| Java | `.java` |
| JavaScript | `.js`, `.jsx` |
| JSON | `.json` |
| PHP | `.php` |
| Python | `.py` |
| Ruby | `.rb` |
| Rust | `.rs` |

The server also recognizes these extensions for detection and project summaries, but parser-backed tools require a bundled or manually registered parser:

| Language | Extensions |
|----------|-----------|
| C# | `.cs` |
| TypeScript | `.ts`, `.tsx` |
| Kotlin | `.kt` |
| Swift | `.swift` |
| Dart | `.dart` |
| Scala | `.scala` |
| Lua | `.lua` |
| Haskell | `.hs` |
| OCaml | `.ml` |
| Elixir | `.ex`, `.exs` |
| Clojure | `.clj` |
| Elm | `.elm` |
| Bash | `.sh` |
| SQL | `.sql` |
| YAML | `.yaml`, `.yml` |
| CSS | `.css` |
| SCSS | `.scss`, `.sass` |
| Markdown | `.md` |
| Protobuf | `.proto` |
| XML | `.xml` |

## Development

```bash
# Build
task build

# Run tests
task test

# Format code
task format

# Lint (requires Docker)
task lint

# Tidy dependencies
task tidy
```

## Demo: Eino Architecture Detective

The repository includes a runnable Eino demo in `demos/eino-architecture-detective`. It launches this MCP server over stdio, uses the MCP tools to gather tree-sitter evidence from a target codebase, and asks an OpenAI-compatible Eino chat model to produce an architecture report.

Create `.env` from `.env.example`, fill in `OPENAI_API_KEY`, then run:

```bash
task demo:eino
```

You can also point it at another project and add a focus question:

```bash
task demo:eino TARGET=/path/to/project FOCUS="Where is the core domain logic?"
```

## License

MIT
