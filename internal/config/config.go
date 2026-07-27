// Package config provides configuration management for the tree-sitter MCP server.
package config

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Transport selects how the MCP server communicates with clients.
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportHTTP  Transport = "http"
)

// RuntimeConfig holds process-level settings for running the MCP server.
type RuntimeConfig struct {
	ConfigPath   string
	Debug        bool
	DisableCache bool
	PreParsePath string
	Transport    Transport
	HTTPAddr     string
	AllowRemote  bool
	ShowVersion  bool
}

// CacheConfig holds caching behavior settings.
type CacheConfig struct {
	Enabled    bool `yaml:"enabled" json:"enabled"`
	MaxSizeMB  int  `yaml:"max_size_mb" json:"max_size_mb"`
	TTLSeconds int  `yaml:"ttl_seconds" json:"ttl_seconds"`
}

// SecurityConfig holds security settings.
type SecurityConfig struct {
	MaxFileSizeMB     int      `yaml:"max_file_size_mb" json:"max_file_size_mb"`
	ExcludedDirs      []string `yaml:"excluded_dirs" json:"excluded_dirs"`
	AllowedExtensions []string `yaml:"allowed_extensions" json:"allowed_extensions,omitempty"`
}

// LanguageConfig holds language-specific settings.
type LanguageConfig struct {
	DefaultMaxDepth    int      `yaml:"default_max_depth" json:"default_max_depth"`
	PreferredLanguages []string `yaml:"preferred_languages" json:"preferred_languages"`
}

// ServerConfig is the main server configuration.
type ServerConfig struct {
	Cache             CacheConfig    `yaml:"cache" json:"cache"`
	Security          SecurityConfig `yaml:"security" json:"security"`
	Language          LanguageConfig `yaml:"language" json:"language"`
	LogLevel          string         `yaml:"log_level" json:"log_level"`
	MaxResultsDefault int            `yaml:"max_results_default" json:"max_results_default"`
}

// DefaultConfig returns a ServerConfig with sensible defaults.
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Cache: CacheConfig{
			Enabled:    true,
			MaxSizeMB:  100,
			TTLSeconds: 300,
		},
		Security: SecurityConfig{
			MaxFileSizeMB: 5,
			ExcludedDirs:  []string{".git", "node_modules", "__pycache__", ".venv", "venv", ".tox"},
		},
		Language: LanguageConfig{
			DefaultMaxDepth: 5,
		},
		LogLevel:          "INFO",
		MaxResultsDefault: 100,
	}
}

// LoadRuntime builds a RuntimeConfig from environment variables, then applies
// command-line flag overrides. Flags take precedence over environment variables.
//
// Environment variables:
//   - MCP_TRANSPORT (stdio|http)
//   - MCP_HTTP_ADDR
//   - MCP_HTTP_ALLOW_REMOTE (true|false)
func LoadRuntime(args []string) (*RuntimeConfig, error) {
	cfg := &RuntimeConfig{
		Transport: TransportStdio,
		HTTPAddr:  "127.0.0.1:8080",
	}

	if v := strings.TrimSpace(os.Getenv("MCP_TRANSPORT")); v != "" {
		cfg.Transport = Transport(strings.ToLower(v))
	}
	if v := strings.TrimSpace(os.Getenv("MCP_HTTP_ADDR")); v != "" {
		cfg.HTTPAddr = v
	}
	if v := strings.TrimSpace(os.Getenv("MCP_HTTP_ALLOW_REMOTE")); v != "" {
		cfg.AllowRemote = strings.EqualFold(v, "true") || v == "1"
	}

	fs := flag.NewFlagSet("tree-sitter-mcp", flag.ContinueOnError)
	configPath := fs.String("config", cfg.ConfigPath, "Path to YAML configuration file")
	debug := fs.Bool("debug", cfg.Debug, "Enable debug logging")
	disableCache := fs.Bool("disable-cache", cfg.DisableCache, "Disable parse tree caching")
	preParsePath := fs.String("pre-parse", cfg.PreParsePath, "Pre-parse all source files in the given directory at startup")
	transport := fs.String("transport", string(cfg.Transport), "MCP transport: stdio or http")
	httpAddr := fs.String("http-addr", cfg.HTTPAddr, "HTTP listen address when using --transport=http")
	allowRemote := fs.Bool("allow-remote-http", cfg.AllowRemote, "Allow binding HTTP transport to non-loopback addresses")
	showVersion := fs.Bool("version", cfg.ShowVersion, "Show version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg.ConfigPath = strings.TrimSpace(*configPath)
	cfg.Debug = *debug
	cfg.DisableCache = *disableCache
	cfg.PreParsePath = strings.TrimSpace(*preParsePath)
	cfg.Transport = Transport(strings.ToLower(strings.TrimSpace(*transport)))
	cfg.HTTPAddr = strings.TrimSpace(*httpAddr)
	cfg.AllowRemote = *allowRemote
	cfg.ShowVersion = *showVersion

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *RuntimeConfig) validate() error {
	if c.Transport != TransportStdio && c.Transport != TransportHTTP {
		return fmt.Errorf("transport must be 'stdio' or 'http', got '%s'", c.Transport)
	}
	if c.Transport == TransportHTTP && !c.AllowRemote && isWildcardListenAddress(c.HTTPAddr) {
		return fmt.Errorf("refusing non-loopback HTTP listen address %q; use --allow-remote-http to override", c.HTTPAddr)
	}
	return nil
}

func isWildcardListenAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Inputs like ":8080" and malformed values should be treated as potentially unsafe.
		return strings.HasPrefix(addr, ":") || addr == ""
	}

	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		return true
	}
	return false
}

// LoadFromFile loads configuration from a YAML file, falling back to defaults.
func LoadFromFile(path string) (*ServerConfig, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return defaults if file doesn't exist
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}

	// Apply environment variable overrides.
	applyEnvOverrides(cfg)
	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides.
func applyEnvOverrides(cfg *ServerConfig) {
	if v := strings.TrimSpace(os.Getenv("TREE_SITTER_MCP_LOG_LEVEL")); v != "" {
		cfg.LogLevel = v
	}
	if v := strings.TrimSpace(os.Getenv("TREE_SITTER_MCP_CACHE_MAX_SIZE_MB")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Cache.MaxSizeMB = n
		}
	}
}

// ConfigurationManager manages runtime configuration updates.
type ConfigurationManager struct {
	config *ServerConfig
}

// NewConfigurationManager creates a new ConfigurationManager with defaults.
func NewConfigurationManager() *ConfigurationManager {
	return &ConfigurationManager{config: DefaultConfig()}
}

// GetConfig returns the current configuration.
func (m *ConfigurationManager) GetConfig() *ServerConfig {
	return m.config
}

// LoadFromFile loads configuration from a file and merges with current config.
func (m *ConfigurationManager) LoadFromFile(path string) error {
	cfg, err := LoadFromFile(path)
	if err != nil {
		return err
	}
	m.config = cfg
	return nil
}

// UpdateValue updates a single configuration value by dotted path.
// Supported paths: "cache.enabled", "cache.max_size_mb", "cache.ttl_seconds",
// "security.max_file_size_mb", "language.default_max_depth", "log_level",
// "max_results_default".
func (m *ConfigurationManager) UpdateValue(path string, value any) {
	switch path {
	case "cache.enabled":
		if v, ok := value.(bool); ok {
			m.config.Cache.Enabled = v
		}
	case "cache.max_size_mb":
		if v, ok := value.(int); ok {
			m.config.Cache.MaxSizeMB = v
		}
	case "cache.ttl_seconds":
		if v, ok := value.(int); ok {
			m.config.Cache.TTLSeconds = v
		}
	case "security.max_file_size_mb":
		if v, ok := value.(int); ok {
			m.config.Security.MaxFileSizeMB = v
		}
	case "language.default_max_depth":
		if v, ok := value.(int); ok {
			m.config.Language.DefaultMaxDepth = v
		}
	case "log_level":
		if v, ok := value.(string); ok {
			m.config.LogLevel = strings.ToUpper(v)
		}
	case "max_results_default":
		if v, ok := value.(int); ok {
			m.config.MaxResultsDefault = v
		}
	}
}

// ToMap converts the configuration to a map for MCP responses.
func (m *ConfigurationManager) ToMap() map[string]any {
	cfg := m.config
	return map[string]any{
		"cache": map[string]any{
			"enabled":     cfg.Cache.Enabled,
			"max_size_mb": cfg.Cache.MaxSizeMB,
			"ttl_seconds": cfg.Cache.TTLSeconds,
		},
		"security": map[string]any{
			"max_file_size_mb":   cfg.Security.MaxFileSizeMB,
			"excluded_dirs":      cfg.Security.ExcludedDirs,
			"allowed_extensions": cfg.Security.AllowedExtensions,
		},
		"language": map[string]any{
			"default_max_depth":   cfg.Language.DefaultMaxDepth,
			"preferred_languages": cfg.Language.PreferredLanguages,
		},
		"log_level":           cfg.LogLevel,
		"max_results_default": cfg.MaxResultsDefault,
	}
}
