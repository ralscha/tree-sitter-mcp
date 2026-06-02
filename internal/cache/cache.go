// Package cache provides parse tree caching for performance optimization.
package cache

import (
	"os"
	"strings"
	"sync"
	"time"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// CachedTree holds a cached parse tree with metadata.
type CachedTree struct {
	Tree       *sitter.Tree
	Source     []byte
	Timestamp  time.Time
	FileSize   int64
	ModifiedAt time.Time
}

// TreeCache provides thread-safe caching of parsed syntax trees.
type TreeCache struct {
	mu        sync.RWMutex
	entries   map[string]*CachedTree
	enabled   bool
	maxSizeMB float64
	ttl       time.Duration
}

// NewTreeCache creates a new tree cache.
func NewTreeCache(maxSizeMB float64, ttlSeconds int) *TreeCache {
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return &TreeCache{
		entries:   make(map[string]*CachedTree),
		enabled:   true,
		maxSizeMB: maxSizeMB,
		ttl:       time.Duration(ttlSeconds) * time.Second,
	}
}

// SetEnabled enables or disables the cache.
func (c *TreeCache) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
	if !enabled {
		c.clearLocked()
	}
}

// SetMaxSizeMB sets the maximum cache size.
func (c *TreeCache) SetMaxSizeMB(maxSizeMB float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxSizeMB = maxSizeMB
	c.evict()
}

// SetTTLSeconds sets the cache entry time-to-live.
func (c *TreeCache) SetTTLSeconds(ttlSeconds int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	c.ttl = time.Duration(ttlSeconds) * time.Second
}

// IsEnabled returns whether caching is enabled.
func (c *TreeCache) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *TreeCache) makeKey(filePath, language string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}
	return language + ":" + filePath + ":" + info.ModTime().String(), nil
}

// Get retrieves a cached tree if available and not expired.
func (c *TreeCache) Get(filePath, language string) (*sitter.Tree, []byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.enabled {
		return nil, nil, false
	}

	key, err := c.makeKey(filePath, language)
	if err != nil {
		return nil, nil, false
	}

	entry, ok := c.entries[key]
	if !ok {
		return nil, nil, false
	}

	if time.Since(entry.Timestamp) > c.ttl {
		return nil, nil, false
	}

	info, err := os.Stat(filePath)
	if err != nil || info.ModTime() != entry.ModifiedAt {
		return nil, nil, false
	}

	return entry.Tree, entry.Source, true
}

// Put stores a parsed tree in the cache.
func (c *TreeCache) Put(filePath, language string, tree *sitter.Tree, source []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.enabled {
		return
	}

	key, err := c.makeKey(filePath, language)
	if err != nil {
		return
	}

	info, _ := os.Stat(filePath)
	modTime := time.Now()
	if info != nil {
		modTime = info.ModTime()
	}

	c.entries[key] = &CachedTree{
		Tree:       tree,
		Source:     source,
		Timestamp:  time.Now(),
		FileSize:   int64(len(source)),
		ModifiedAt: modTime,
	}

	c.evict()
}

// Invalidate clears cache entries.
func (c *TreeCache) Invalidate(filePath ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(filePath) == 0 || filePath[0] == "" {
		c.clearLocked()
		return
	}

	for key := range c.entries {
		if containsPath(key, filePath[0]) {
			c.entries[key].Close()
			delete(c.entries, key)
		}
	}
}

// Close releases all cached tree resources.
func (c *TreeCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearLocked()
}

func (c *TreeCache) clearLocked() {
	for _, entry := range c.entries {
		entry.Close()
	}
	c.entries = make(map[string]*CachedTree)
}

// Close releases the cached tree, if present.
func (ct *CachedTree) Close() {
	if ct != nil && ct.Tree != nil {
		ct.Tree.Close()
		ct.Tree = nil
	}
}

func containsPath(key, path string) bool {
	return strings.Contains(key, path)
}

func (c *TreeCache) evict() {
	maxBytes := int64(c.maxSizeMB * 1024 * 1024)
	var totalSize int64
	for _, entry := range c.entries {
		totalSize += entry.FileSize
	}

	for totalSize > maxBytes && len(c.entries) > 0 {
		var oldestKey string
		var oldestTime time.Time
		first := true
		for key, entry := range c.entries {
			if first || entry.Timestamp.Before(oldestTime) {
				oldestKey = key
				oldestTime = entry.Timestamp
				first = false
			}
		}
		if oldestKey != "" {
			totalSize -= c.entries[oldestKey].FileSize
			c.entries[oldestKey].Close()
			delete(c.entries, oldestKey)
		}
	}
}
