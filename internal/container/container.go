// Package container provides dependency injection for the tree-sitter MCP server.
package container

import (
	"tree-sitter-mcp/internal/cache"
	"tree-sitter-mcp/internal/config"
	"tree-sitter-mcp/internal/language"
	"tree-sitter-mcp/internal/models"
	"tree-sitter-mcp/internal/tools"
)

// Container holds all application dependencies.
type Container struct {
	ConfigManager    *config.ConfigurationManager
	ProjectRegistry  *models.ProjectRegistry
	LanguageRegistry *language.Registry
	TreeCache        *cache.TreeCache
}

// NewContainer creates a new dependency container with all services initialized.
func NewContainer() *Container {
	cfgMgr := config.NewConfigurationManager()
	cfg := cfgMgr.GetConfig()

	langReg := language.NewRegistry()
	projReg := models.NewProjectRegistry()
	treeCache := cache.NewTreeCache(
		float64(cfg.Cache.MaxSizeMB),
		cfg.Cache.TTLSeconds,
	)

	c := &Container{
		ConfigManager:    cfgMgr,
		ProjectRegistry:  projReg,
		LanguageRegistry: langReg,
		TreeCache:        treeCache,
	}
	c.ApplyConfig()
	return c
}

// GetConfig returns the current configuration.
func (c *Container) GetConfig() *config.ServerConfig {
	return c.ConfigManager.GetConfig()
}

// ApplyConfig propagates current configuration to runtime services.
func (c *Container) ApplyConfig() {
	cfg := c.GetConfig()
	c.TreeCache.SetEnabled(cfg.Cache.Enabled)
	c.TreeCache.SetMaxSizeMB(float64(cfg.Cache.MaxSizeMB))
	c.TreeCache.SetTTLSeconds(cfg.Cache.TTLSeconds)
	tools.SetSecurityLimits(cfg.Security.MaxFileSizeMB, cfg.Security.AllowedExtensions)
}

// Close releases resources owned by the container.
func (c *Container) Close() {
	c.TreeCache.Close()
	c.LanguageRegistry.Close()
}
