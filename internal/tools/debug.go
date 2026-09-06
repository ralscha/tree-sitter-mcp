// Package tools provides debug utilities for the MCP server.
package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"tree-sitter-mcp/internal/config"

	"gopkg.in/yaml.v3"
)

// DiagnoseConfigResult holds diagnostic information about a YAML configuration file.
type DiagnoseConfigResult struct {
	FilePath     string         `json:"file_path"`
	Exists       bool           `json:"exists"`
	Readable     bool           `json:"readable"`
	YamlValid    bool           `json:"yaml_valid"`
	ParsedData   map[string]any `json:"parsed_data,omitempty"`
	ConfigBefore map[string]any `json:"config_before,omitempty"`
	ConfigAfter  map[string]any `json:"config_after,omitempty"`
	Error        *string        `json:"error,omitempty"`
}

// DiagnoseYamlConfig diagnoses issues with a YAML configuration file.
func DiagnoseYamlConfig(configPath string, cfgMgr *config.ConfigurationManager) *DiagnoseConfigResult {
	result := &DiagnoseConfigResult{
		FilePath: configPath,
	}

	// Check if file exists.
	info, err := os.Stat(configPath)
	if err != nil {
		errStr := fmt.Sprintf("file does not exist: %s", configPath)
		result.Error = &errStr
		return result
	}
	result.Exists = true

	// Check if file is readable.
	if info.IsDir() {
		errStr := fmt.Sprintf("path is a directory: %s", configPath)
		result.Error = &errStr
		return result
	}

	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		errStr := fmt.Sprintf("error reading file: %v", err)
		result.Error = &errStr
		return result
	}
	result.Readable = true

	// Try to parse YAML.
	var parsedData map[string]any
	if err := yaml.Unmarshal(data, &parsedData); err != nil {
		errStr := fmt.Sprintf("error parsing YAML: %v", err)
		result.Error = &errStr
		return result
	}

	if parsedData == nil {
		errStr := "YAML parser returned nil (file empty or contains only comments)"
		result.Error = &errStr
		return result
	}
	result.YamlValid = true
	result.ParsedData = parsedData

	// Capture config before loading.
	cfgBefore := cfgMgr.GetConfig()
	result.ConfigBefore = map[string]any{
		"cache.max_size_mb":          cfgBefore.Cache.MaxSizeMB,
		"security.max_file_size_mb":  cfgBefore.Security.MaxFileSizeMB,
		"language.default_max_depth": cfgBefore.Language.DefaultMaxDepth,
	}

	// Load into a temporary value. Diagnostics are read-only and must not mutate
	// the server's active configuration.
	cfgAfter, loadErr := config.LoadFromFile(configPath)
	if loadErr != nil {
		errStr := fmt.Sprintf("error loading config: %v", loadErr)
		result.Error = &errStr
		return result
	}

	// Capture the configuration that would result from loading the file.
	result.ConfigAfter = map[string]any{
		"cache.max_size_mb":          cfgAfter.Cache.MaxSizeMB,
		"security.max_file_size_mb":  cfgAfter.Security.MaxFileSizeMB,
		"language.default_max_depth": cfgAfter.Language.DefaultMaxDepth,
	}

	return result
}
