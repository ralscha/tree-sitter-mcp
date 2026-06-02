# Eino Architecture Detective

This demo uses Eino to connect to the `tree-sitter-mcp` server over stdio, gathers structural facts about a codebase with MCP tools, and asks an OpenAI-compatible chat model to write an evidence-backed architecture tour.

It is intentionally a little more interesting than a tool-listing sample: it registers a project, analyzes language mix, pulls files, extracts symbols, checks dependencies and complexity, runs a tree-sitter query, and asks the model to identify likely entry points, coupling patterns, and refactor targets from those facts.

## Setup

Create a `.env` file in the repository root:

```sh
cp .env.example .env
```

Fill in at least:

```sh
OPENAI_API_KEY=...
OPENAI_MODEL=gpt-5.4-mini
```

Optional model settings:

- `OPENAI_BASE_URL`
- `OPENAI_BY_AZURE=true`
- `OPENAI_API_VERSION`

Optional server setting:

- `TREE_SITTER_MCP_BIN=bin/tree-sitter-mcp.exe`

The easiest path is to use the root `Taskfile.yml`, which builds the MCP server and points the demo at that binary.

## Run

Analyze this repository:

```sh
task demo:eino
```

Analyze another project:

```sh
task demo:eino TARGET=C:/path/to/project FOCUS="Where is the core business logic?"
```

You can also run the demo module directly:

```sh
cd demos/eino-architecture-detective
TREE_SITTER_MCP_BIN=../../bin/tree-sitter-mcp go run . --env ../../.env ../..
```
