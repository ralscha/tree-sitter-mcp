// Command tree-sitter-mcp is an MCP server that provides code analysis
// capabilities using tree-sitter, designed to give AI assistants intelligent
// access to codebases with appropriate context management.
//
// Usage:
//
//	tree-sitter-mcp [flags]
//
// Flags:
//
//	--config string       Path to YAML configuration file
//	--debug               Enable debug logging
//	--disable-cache       Disable parse tree caching
//	--pre-parse string    Pre-parse all source files in a directory at startup
//	--version             Show version and exit
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"tree-sitter-mcp/internal/config"
	"tree-sitter-mcp/internal/server"
	"tree-sitter-mcp/internal/tools"
)

const version = "0.1.0"

func main() {
	configPath := flag.String("config", "", "Path to YAML configuration file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	disableCache := flag.Bool("disable-cache", false, "Disable parse tree caching")
	preParsePath := flag.String("pre-parse", "", "Pre-parse all source files in the given directory at startup")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("tree-sitter-mcp version %s\n", version)
		os.Exit(0)
	}

	if *debug {
		_ = os.Setenv("MCP_TS_LOG_LEVEL", "DEBUG")
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		log.Println("Debug logging enabled")
	}

	// Create the MCP server.
	srv := server.NewMCPServer()
	defer srv.GetContainer().Close()

	// Apply command-line configuration overrides.
	cfgMgr := srv.GetContainer().ConfigManager

	if *configPath != "" {
		log.Printf("Loading configuration from %s\n", *configPath)
		if err := cfgMgr.LoadFromFile(*configPath); err != nil {
			log.Printf("Warning: failed to load config: %v\n", err)
		} else {
			srv.GetContainer().ApplyConfig()
		}
	}

	if *disableCache {
		log.Println("Disabling parse tree cache")
		cfgMgr.UpdateValue("cache.enabled", false)
		srv.GetContainer().TreeCache.SetEnabled(false)
	}

	if *debug {
		cfgMgr.UpdateValue("log_level", "DEBUG")
		srv.GetContainer().ApplyConfig()
	}

	// Pre-parse a project directory if requested.
	if *preParsePath != "" {
		container := srv.GetContainer()
		cfg := container.GetConfig()
		log.Printf("Pre-parsing project at %s ...\n", *preParsePath)
		result, err := tools.PreParseProject(
			*preParsePath,
			container.LanguageRegistry,
			container.TreeCache,
			cfg.Security.ExcludedDirs,
		)
		if err != nil {
			log.Printf("Pre-parse warning: %v\n", err)
		}
		if result != nil {
			log.Printf("Pre-parse complete: %d files scanned, %d parsed, %d skipped, %d errors in %.1fs\n",
				result.TotalFiles, result.Parsed, result.Skipped, result.Errors, result.ElapsedSecs)
			for lang, count := range result.ByLanguage {
				log.Printf("  %s: %d files\n", lang, count)
			}
		}
	}

	// Log startup configuration.
	cfg := cfgMgr.GetConfig()
	log.Printf("Starting tree-sitter MCP server (cache: %v, max_file_size: %dMB, max_depth: %d)\n",
		cfg.Cache.Enabled, cfg.Security.MaxFileSizeMB, cfg.Language.DefaultMaxDepth)

	// Run the server over stdio.
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}

// Ensure config is used.
var _ = config.DefaultConfig
