package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Cache.Enabled {
		t.Error("cache should be enabled by default")
	}
	if cfg.Cache.MaxSizeMB != 100 {
		t.Errorf("cache.max_size_mb = %d, want 100", cfg.Cache.MaxSizeMB)
	}
	if cfg.Cache.TTLSeconds != 300 {
		t.Errorf("cache.ttl_seconds = %d, want 300", cfg.Cache.TTLSeconds)
	}
	if cfg.Security.MaxFileSizeMB != 5 {
		t.Errorf("security.max_file_size_mb = %d, want 5", cfg.Security.MaxFileSizeMB)
	}
	if len(cfg.Security.ExcludedDirs) < 3 {
		t.Error("excluded_dirs should have at least 3 entries by default")
	}
	if cfg.Language.DefaultMaxDepth != 5 {
		t.Errorf("language.default_max_depth = %d, want 5", cfg.Language.DefaultMaxDepth)
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("log_level = %s, want INFO", cfg.LogLevel)
	}
	if cfg.MaxResultsDefault != 100 {
		t.Errorf("max_results_default = %d, want 100", cfg.MaxResultsDefault)
	}
}

func TestConfigurationManagerUpdateValue(t *testing.T) {
	mgr := NewConfigurationManager()

	// Update bool.
	mgr.UpdateValue("cache.enabled", false)
	if mgr.config.Cache.Enabled {
		t.Error("cache.enabled should be false after update")
	}

	// Update int.
	mgr.UpdateValue("cache.max_size_mb", 256)
	if mgr.config.Cache.MaxSizeMB != 256 {
		t.Errorf("cache.max_size_mb = %d, want 256", mgr.config.Cache.MaxSizeMB)
	}

	mgr.UpdateValue("cache.ttl_seconds", 600)
	if mgr.config.Cache.TTLSeconds != 600 {
		t.Errorf("cache.ttl_seconds = %d, want 600", mgr.config.Cache.TTLSeconds)
	}

	mgr.UpdateValue("security.max_file_size_mb", 10)
	if mgr.config.Security.MaxFileSizeMB != 10 {
		t.Errorf("security.max_file_size_mb = %d, want 10", mgr.config.Security.MaxFileSizeMB)
	}

	mgr.UpdateValue("language.default_max_depth", 7)
	if mgr.config.Language.DefaultMaxDepth != 7 {
		t.Errorf("language.default_max_depth = %d, want 7", mgr.config.Language.DefaultMaxDepth)
	}

	// Update string.
	mgr.UpdateValue("log_level", "DEBUG")
	if mgr.config.LogLevel != "DEBUG" {
		t.Errorf("log_level = %s, want DEBUG", mgr.config.LogLevel)
	}

	mgr.UpdateValue("max_results_default", 200)
	if mgr.config.MaxResultsDefault != 200 {
		t.Errorf("max_results_default = %d, want 200", mgr.config.MaxResultsDefault)
	}
}

func TestConfigurationManagerUpdateValueInvalidType(t *testing.T) {
	mgr := NewConfigurationManager()

	// Passing string to int field should be a no-op.
	before := mgr.config.Cache.MaxSizeMB
	mgr.UpdateValue("cache.max_size_mb", "not_an_int")
	if mgr.config.Cache.MaxSizeMB != before {
		t.Error("cache.max_size_mb should not change with invalid type")
	}

	// Passing int to bool field should be a no-op.
	beforeBool := mgr.config.Cache.Enabled
	mgr.UpdateValue("cache.enabled", 1)
	if mgr.config.Cache.Enabled != beforeBool {
		t.Error("cache.enabled should not change with invalid type")
	}
}

func TestConfigurationManagerUpdateValueUnknownPath(t *testing.T) {
	mgr := NewConfigurationManager()

	// Unknown paths should be silently ignored.
	cfgBefore := *mgr.config
	mgr.UpdateValue("unknown.path", true)
	if mgr.config.Cache.Enabled != cfgBefore.Cache.Enabled {
		t.Error("config should be unchanged for unknown path")
	}
}

func TestConfigurationManagerGetConfig(t *testing.T) {
	mgr := NewConfigurationManager()
	cfg := mgr.GetConfig()

	if cfg == nil {
		t.Fatal("GetConfig returned nil")
	}
	if !cfg.Cache.Enabled {
		t.Error("default config should have cache enabled")
	}
}

func TestConfigurationManagerToMap(t *testing.T) {
	mgr := NewConfigurationManager()
	m := mgr.ToMap()

	if m["cache"] == nil {
		t.Error("ToMap should include cache section")
	}
	if m["security"] == nil {
		t.Error("ToMap should include security section")
	}
	if m["language"] == nil {
		t.Error("ToMap should include language section")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlData := map[string]any{
		"cache": map[string]any{
			"enabled":     false,
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

	data, err := yaml.Marshal(yamlData)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write YAML: %v", err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.Cache.Enabled {
		t.Error("cache.enabled should be false from YAML")
	}
	if cfg.Cache.MaxSizeMB != 256 {
		t.Errorf("cache.max_size_mb = %d, want 256", cfg.Cache.MaxSizeMB)
	}
	if cfg.Cache.TTLSeconds != 600 {
		t.Errorf("cache.ttl_seconds = %d, want 600", cfg.Cache.TTLSeconds)
	}
	if cfg.Security.MaxFileSizeMB != 10 {
		t.Errorf("security.max_file_size_mb = %d, want 10", cfg.Security.MaxFileSizeMB)
	}
	if cfg.Language.DefaultMaxDepth != 7 {
		t.Errorf("language.default_max_depth = %d, want 7", cfg.Language.DefaultMaxDepth)
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("log_level = %s, want DEBUG", cfg.LogLevel)
	}
	if cfg.MaxResultsDefault != 200 {
		t.Errorf("max_results_default = %d, want 200", cfg.MaxResultsDefault)
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	// LoadFromFile returns defaults if file doesn't exist.
	cfg, err := LoadFromFile("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadFromFile should not error for missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFromFile returned nil")
	}
	if !cfg.Cache.Enabled {
		t.Error("missing file should return defaults with cache enabled")
	}
}

func TestConfigurationManagerLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlData := map[string]any{
		"cache": map[string]any{"enabled": false},
	}
	data, _ := yaml.Marshal(yamlData)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewConfigurationManager()
	if err := mgr.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if mgr.config.Cache.Enabled {
		t.Error("cache.enabled should be false after loading from file")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("MCP_TS_LOG_LEVEL", "ERROR")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.LogLevel != "ERROR" {
		t.Errorf("log_level = %s, want ERROR", cfg.LogLevel)
	}
}
