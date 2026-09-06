package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTreeCache(t *testing.T) {
	c := NewTreeCache(100, 300)
	if c == nil {
		t.Fatal("NewTreeCache returned nil")
	}
	if !c.enabled {
		t.Error("cache should be enabled by default")
	}
	if c.maxSizeMB != 100 {
		t.Errorf("maxSizeMB = %v, want 100", c.maxSizeMB)
	}
	if c.ttl != 300*time.Second {
		t.Errorf("ttl = %v, want 300s", c.ttl)
	}
}

func TestNewTreeCacheDefaults(t *testing.T) {
	c := NewTreeCache(0, 0)
	if c.maxSizeMB != 100 {
		t.Errorf("maxSizeMB = %v, want 100 (default)", c.maxSizeMB)
	}
	if c.ttl != 300*time.Second {
		t.Errorf("ttl = %v, want 300s (default)", c.ttl)
	}
}

func TestSetEnabled(t *testing.T) {
	c := NewTreeCache(100, 300)

	c.SetEnabled(false)
	if c.IsEnabled() {
		t.Error("cache should be disabled after SetEnabled(false)")
	}
	if len(c.entries) != 0 {
		t.Error("entries should be cleared when disabling")
	}

	c.SetEnabled(true)
	if !c.IsEnabled() {
		t.Error("cache should be enabled after SetEnabled(true)")
	}
}

func TestSetMaxSizeMB(t *testing.T) {
	c := NewTreeCache(100, 300)
	c.SetMaxSizeMB(50)

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.maxSizeMB != 50 {
		t.Errorf("maxSizeMB = %v, want 50", c.maxSizeMB)
	}
}

func TestInvalidateAll(t *testing.T) {
	c := NewTreeCache(100, 300)

	// Manually insert an entry.
	c.mu.Lock()
	c.entries["test:key"] = &CachedTree{Timestamp: time.Now()}
	c.mu.Unlock()

	if len(c.entries) != 1 {
		t.Fatal("entry should exist before invalidation")
	}

	c.Invalidate()
	if len(c.entries) != 0 {
		t.Error("entries should be cleared after Invalidate()")
	}
}

func TestInvalidateSpecific(t *testing.T) {
	c := NewTreeCache(100, 300)

	c.mu.Lock()
	c.entries["a"] = &CachedTree{Timestamp: time.Now()}
	c.entries["b"] = &CachedTree{Timestamp: time.Now()}
	c.mu.Unlock()

	c.Invalidate("a")

	c.mu.RLock()
	_, aExists := c.entries["a"]
	_, bExists := c.entries["b"]
	c.mu.RUnlock()

	if aExists {
		t.Error("entry 'a' should be invalidated")
	}
	if !bExists {
		t.Error("entry 'b' should still exist")
	}
}

func TestInvalidateSpecificDoesNotUseSubstringMatching(t *testing.T) {
	c := NewTreeCache(100, 300)

	c.mu.Lock()
	c.entries["one"] = &CachedTree{Timestamp: time.Now(), FilePath: filepath.Join("root", "file.go")}
	c.entries["two"] = &CachedTree{Timestamp: time.Now(), FilePath: filepath.Join("root", "file.go.bak")}
	c.mu.Unlock()

	c.Invalidate(filepath.Join("root", "file.go"))
	if _, exists := c.entries["one"]; exists {
		t.Fatal("exact cache entry was not invalidated")
	}
	if _, exists := c.entries["two"]; !exists {
		t.Fatal("substring-related cache entry was incorrectly invalidated")
	}
}

func TestPutReplacesEntryInsteadOfAccumulatingByModTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte("package one"), 0644); err != nil {
		t.Fatal(err)
	}
	c := NewTreeCache(100, 300)
	c.Put(path, "go", nil, []byte("package one"))

	if err := os.WriteFile(path, []byte("package two_longer"), 0644); err != nil {
		t.Fatal(err)
	}
	c.Put(path, "go", nil, []byte("package two_longer"))

	if len(c.entries) != 1 {
		t.Fatalf("cache contains %d entries for one file, want 1", len(c.entries))
	}
}

func TestInvalidateEmptyString(t *testing.T) {
	c := NewTreeCache(100, 300)

	c.mu.Lock()
	c.entries["a"] = &CachedTree{Timestamp: time.Now()}
	c.mu.Unlock()

	c.Invalidate("")
	if len(c.entries) != 0 {
		t.Error("empty string should invalidate all")
	}
}

func TestGetDisabled(t *testing.T) {
	c := NewTreeCache(100, 300)
	c.SetEnabled(false)

	_, _, ok := c.Get("/nonexistent", "python")
	if ok {
		t.Error("Get should return false when cache is disabled")
	}
}

func TestGetNonExistent(t *testing.T) {
	c := NewTreeCache(100, 300)

	_, _, ok := c.Get("/nonexistent/file.py", "python")
	if ok {
		t.Error("Get should return false for non-existent file")
	}
}

func TestPutAndGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.py")
	if err := os.WriteFile(path, []byte("x = 1"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	c := NewTreeCache(100, 300)

	// Put a nil tree (simulating a placeholder entry for testing).
	c.Put(path, "python", nil, []byte("x = 1"))

	_, source, ok := c.Get(path, "python")
	if !ok {
		t.Error("Get should return true for cached entry")
	}
	if string(source) != "x = 1" {
		t.Errorf("source = %s, want 'x = 1'", string(source))
	}
}

func TestPutDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.py")
	if err := os.WriteFile(path, []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewTreeCache(100, 300)
	c.SetEnabled(false)

	c.Put(path, "python", nil, []byte("x = 1"))

	if len(c.entries) != 0 {
		t.Error("Put should not store when cache is disabled")
	}
}

func TestMakeKey(t *testing.T) {
	c := NewTreeCache(100, 300)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	key, err := c.makeKey(path, "go")
	if err != nil {
		t.Fatalf("makeKey failed: %v", err)
	}
	if key == "" {
		t.Error("makeKey returned empty string")
	}

	// Same file should produce same key.
	key2, _ := c.makeKey(path, "go")
	if key != key2 {
		t.Error("makeKey should be deterministic for same file")
	}

	// Different language should produce different key.
	key3, _ := c.makeKey(path, "rust")
	if key == key3 {
		t.Error("makeKey should differ for different languages")
	}
}

func TestMakeKeyNonExistent(t *testing.T) {
	c := NewTreeCache(100, 300)

	_, err := c.makeKey("/nonexistent/path.py", "python")
	if err == nil {
		t.Error("makeKey should fail for non-existent file")
	}
}
