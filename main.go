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
//	--transport string    MCP transport: stdio or http
//	--http-addr string    HTTP listen address when using HTTP
//	--version             Show version and exit
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"tree-sitter-mcp/internal/config"
	"tree-sitter-mcp/internal/server"
	"tree-sitter-mcp/internal/tools"
)

const version = "0.1.0"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	runtimeCfg, err := config.LoadRuntime(os.Args[1:])
	if err != nil {
		return err
	}

	if runtimeCfg.ShowVersion {
		fmt.Printf("tree-sitter-mcp version %s\n", version)
		return nil
	}

	if runtimeCfg.Debug {
		_ = os.Setenv("TREE_SITTER_MCP_LOG_LEVEL", "DEBUG")
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		log.Println("Debug logging enabled")
	}

	// Create the MCP server.
	srv := server.NewMCPServer()
	defer srv.GetContainer().Close()

	// Apply command-line configuration overrides.
	cfgMgr := srv.GetContainer().ConfigManager

	if runtimeCfg.ConfigPath != "" {
		log.Printf("Loading configuration from %q\n", runtimeCfg.ConfigPath)
		if err := cfgMgr.LoadFromFile(runtimeCfg.ConfigPath); err != nil {
			log.Printf("Warning: failed to load config: %q\n", err.Error())
		} else {
			srv.GetContainer().ApplyConfig()
		}
	}

	if runtimeCfg.DisableCache {
		log.Println("Disabling parse tree cache")
		cfgMgr.UpdateValue("cache.enabled", false)
		srv.GetContainer().TreeCache.SetEnabled(false)
	}

	if runtimeCfg.Debug {
		cfgMgr.UpdateValue("log_level", "DEBUG")
		srv.GetContainer().ApplyConfig()
	}

	// Pre-parse a project directory if requested.
	if runtimeCfg.PreParsePath != "" {
		container := srv.GetContainer()
		cfg := container.GetConfig()
		log.Printf("Pre-parsing project at %q ...\n", runtimeCfg.PreParsePath)
		result, err := tools.PreParseProject(
			runtimeCfg.PreParsePath,
			container.LanguageRegistry,
			container.TreeCache,
			cfg.Security.ExcludedDirs,
		)
		if err != nil {
			log.Printf("Pre-parse warning: %q\n", err.Error())
		}
		if result != nil {
			log.Printf("Pre-parse complete: %d files scanned, %d parsed, %d skipped, %d errors in %.1fs\n",
				result.TotalFiles, result.Parsed, result.Skipped, result.Errors, result.ElapsedSecs)
			for lang, count := range result.ByLanguage {
				log.Printf("  %q: %d files\n", lang, count)
			}
		}
	}

	// Log startup configuration.
	cfg := cfgMgr.GetConfig()
	runOpts := server.RunOptions{
		Transport: server.Transport(runtimeCfg.Transport),
		HTTPAddr:  runtimeCfg.HTTPAddr,
	}
	log.Printf("Starting tree-sitter MCP server (cache: %v, max_file_size: %dMB, max_depth: %d, transport: %q)\n",
		cfg.Cache.Enabled, cfg.Security.MaxFileSizeMB, cfg.Language.DefaultMaxDepth, runOpts.Transport)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.RunWithOptions(ctx, runOpts); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
