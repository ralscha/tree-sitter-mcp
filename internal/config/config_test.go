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

func TestLoadRuntimeDefaults(t *testing.T) {
	clearRuntimeEnv(t)

	cfg, err := LoadRuntime(nil)
	if err != nil {
		t.Fatalf("LoadRuntime failed: %v", err)
	}

	if cfg.Transport != TransportStdio {
		t.Errorf("transport = %s, want %s", cfg.Transport, TransportStdio)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Errorf("http_addr = %s, want 127.0.0.1:8080", cfg.HTTPAddr)
	}
}

func TestLoadRuntimeEnv(t *testing.T) {
	clearRuntimeEnv(t)
	t.Setenv("MCP_TRANSPORT", "http")
	t.Setenv("MCP_HTTP_ADDR", "127.0.0.1:9090")

	cfg, err := LoadRuntime(nil)
	if err != nil {
		t.Fatalf("LoadRuntime failed: %v", err)
	}

	if cfg.Transport != TransportHTTP {
		t.Errorf("transport = %s, want %s", cfg.Transport, TransportHTTP)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" {
		t.Errorf("http_addr = %s, want 127.0.0.1:9090", cfg.HTTPAddr)
	}
}

func TestLoadRuntimeFlagOverride(t *testing.T) {
	clearRuntimeEnv(t)
	t.Setenv("MCP_TRANSPORT", "stdio")
	t.Setenv("MCP_HTTP_ADDR", "127.0.0.1:8080")

	cfg, err := LoadRuntime([]string{
		"--config=config.yaml",
		"--debug",
		"--disable-cache",
		"--pre-parse=.",
		"--transport=http",
		"--http-addr=127.0.0.1:6060",
	})
	if err != nil {
		t.Fatalf("LoadRuntime failed: %v", err)
	}

	if cfg.ConfigPath != "config.yaml" {
		t.Errorf("config_path = %s, want config.yaml", cfg.ConfigPath)
	}
	if !cfg.Debug {
		t.Error("debug should be true")
	}
	if !cfg.DisableCache {
		t.Error("disable_cache should be true")
	}
	if cfg.PreParsePath != "." {
		t.Errorf("pre_parse_path = %s, want .", cfg.PreParsePath)
	}
	if cfg.Transport != TransportHTTP {
		t.Errorf("transport = %s, want %s", cfg.Transport, TransportHTTP)
	}
	if cfg.HTTPAddr != "127.0.0.1:6060" {
		t.Errorf("http_addr = %s, want 127.0.0.1:6060", cfg.HTTPAddr)
	}
}

func TestLoadRuntimeRejectsWildcardHTTPByDefault(t *testing.T) {
	clearRuntimeEnv(t)

	if _, err := LoadRuntime([]string{"--transport=http", "--http-addr=:8080"}); err == nil {
		t.Fatal("expected wildcard HTTP bind to be rejected without explicit opt-in")
	}
}

func TestLoadRuntimeAllowsWildcardHTTPWithOptIn(t *testing.T) {
	clearRuntimeEnv(t)

	cfg, err := LoadRuntime([]string{"--transport=http", "--http-addr=:8080", "--allow-remote-http"})
	if err != nil {
		t.Fatalf("LoadRuntime failed: %v", err)
	}
	if !cfg.AllowRemote {
		t.Fatal("allow_remote should be true")
	}
}

func TestLoadRuntimeRejectsNonLoopbackHTTPByDefault(t *testing.T) {
	clearRuntimeEnv(t)

	if _, err := LoadRuntime([]string{"--transport=http", "--http-addr=192.168.1.20:8080"}); err == nil {
		t.Fatal("expected non-loopback HTTP bind to be rejected without explicit opt-in")
	}
}

func TestLoadRuntimeAllowsLoopbackHTTPAddresses(t *testing.T) {
	clearRuntimeEnv(t)

	for _, addr := range []string{"localhost:8080", "127.0.0.2:8080", "[::1]:8080"} {
		if _, err := LoadRuntime([]string{"--transport=http", "--http-addr=" + addr}); err != nil {
			t.Errorf("LoadRuntime rejected loopback address %q: %v", addr, err)
		}
	}
}

func TestLoadRuntimeInvalidTransport(t *testing.T) {
	clearRuntimeEnv(t)

	if _, err := LoadRuntime([]string{"--transport=sse"}); err == nil {
		t.Fatal("expected invalid transport error")
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
	if _, err := LoadFromFile("/nonexistent/path/config.yaml"); err == nil {
		t.Fatal("LoadFromFile should report an explicitly requested missing file")
	}
}

func TestLoadFromFileRejectsInvalidValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("cache:\n  max_size_mb: 0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFromFile(path); err == nil {
		t.Fatal("LoadFromFile should reject non-positive cache size")
	}
}

func TestLoadFromFileRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("cache:\n  max_sze_mb: 10\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFromFile(path); err == nil {
		t.Fatal("LoadFromFile should reject unknown fields instead of ignoring likely typos")
	}
}

func TestGetConfigReturnsIndependentCopy(t *testing.T) {
	mgr := NewConfigurationManager()
	copy := mgr.GetConfig()
	copy.Cache.Enabled = false
	copy.Security.ExcludedDirs[0] = "changed"

	got := mgr.GetConfig()
	if !got.Cache.Enabled || got.Security.ExcludedDirs[0] == "changed" {
		t.Fatal("GetConfig exposed mutable internal configuration")
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
	t.Setenv("TREE_SITTER_MCP_LOG_LEVEL", "ERROR")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.LogLevel != "ERROR" {
		t.Errorf("log_level = %s, want ERROR", cfg.LogLevel)
	}
}

func TestConfigurationManagerAppliesEnvironmentWithoutConfigFile(t *testing.T) {
	t.Setenv("TREE_SITTER_MCP_LOG_LEVEL", "debug")
	t.Setenv("TREE_SITTER_MCP_CACHE_MAX_SIZE_MB", "42")

	cfg := NewConfigurationManager().GetConfig()
	if cfg.LogLevel != "DEBUG" || cfg.Cache.MaxSizeMB != 42 {
		t.Fatalf("environment overrides not applied to defaults: %#v", cfg)
	}
}

func clearRuntimeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_HTTP_ADDR", "")
	t.Setenv("MCP_HTTP_ALLOW_REMOTE", "")
}
