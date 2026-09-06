package tools

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"tree-sitter-mcp/internal/config"
)

func TestDiagnoseYamlConfigFileNotFound(t *testing.T) {
	cfgMgr := config.NewConfigurationManager()
	result := DiagnoseYamlConfig("/nonexistent/path/config.yaml", cfgMgr)

	if result.Exists {
		t.Error("Exists should be false for non-existent file")
	}
	if result.Error == nil {
		t.Error("Error should be set for non-existent file")
	}
}

func TestDiagnoseYamlConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	cfgMgr := config.NewConfigurationManager()
	result := DiagnoseYamlConfig(dir, cfgMgr)

	// Directories return true for os.Stat, but should fail readability.
	if result.Error == nil {
		t.Error("Error should be set for directory path")
	}
}

func TestDiagnoseYamlConfigValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlData := map[string]any{
		"cache": map[string]any{
			"enabled":     true,
			"max_size_mb": 256,
			"ttl_seconds": 600,
		},
		"security": map[string]any{
			"max_file_size_mb": 10,
		},
		"language": map[string]any{
			"default_max_depth": 7,
		},
		"log_level":           "DEBUG",
		"max_results_default": 200,
	}
	data, _ := yaml.Marshal(yamlData)
	os.WriteFile(path, data, 0644)

	cfgMgr := config.NewConfigurationManager()
	result := DiagnoseYamlConfig(path, cfgMgr)

	if !result.Exists {
		t.Error("Exists should be true for valid file")
	}
	if !result.Readable {
		t.Error("Readable should be true for valid file")
	}
	if !result.YamlValid {
		t.Error("YamlValid should be true for valid YAML")
	}
	if result.Error != nil {
		t.Errorf("Error should be nil, got: %s", *result.Error)
	}
	if result.ParsedData == nil {
		t.Error("ParsedData should not be nil")
	}
	if result.ConfigBefore == nil {
		t.Error("ConfigBefore should not be nil")
	}
	if result.ConfigAfter == nil {
		t.Error("ConfigAfter should not be nil")
	}

	// Config after should reflect the loaded values.
	after := result.ConfigAfter
	if after["cache.max_size_mb"].(int) != 256 {
		t.Errorf("cache.max_size_mb = %v, want 256", after["cache.max_size_mb"])
	}
	if after["language.default_max_depth"].(int) != 7 {
		t.Errorf("language.default_max_depth = %v, want 7", after["language.default_max_depth"])
	}
	if got := cfgMgr.GetConfig().Cache.MaxSizeMB; got != 100 {
		t.Fatalf("diagnostics mutated active config: cache.max_size_mb = %d, want 100", got)
	}
}

func TestDiagnoseYamlConfigInvalidYaml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yaml")
	os.WriteFile(path, []byte(": invalid: yaml: :"), 0644)

	cfgMgr := config.NewConfigurationManager()
	result := DiagnoseYamlConfig(path, cfgMgr)

	if !result.Exists {
		t.Error("Exists should be true")
	}
	if !result.Readable {
		t.Error("Readable should be true")
	}
	if result.YamlValid {
		t.Error("YamlValid should be false for invalid YAML")
	}
	if result.Error == nil {
		t.Error("Error should be set for invalid YAML")
	}
}

func TestDiagnoseYamlConfigEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	os.WriteFile(path, []byte(""), 0644)

	cfgMgr := config.NewConfigurationManager()
	result := DiagnoseYamlConfig(path, cfgMgr)

	if !result.Exists {
		t.Error("Exists should be true")
	}
	if result.YamlValid {
		t.Error("YamlValid should be false for empty/comment-only YAML")
	}
	if result.Error == nil {
		t.Error("Error should be set for empty YAML")
	}
}
