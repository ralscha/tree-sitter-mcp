package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunWithOptionsHTTPShutsDown(t *testing.T) {
	srv := NewMCPServer()
	defer srv.GetContainer().Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := srv.RunWithOptions(ctx, RunOptions{
		Transport: HTTPTransport,
		HTTPAddr:  "127.0.0.1:0",
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
}

func TestRunWithOptionsInvalidTransport(t *testing.T) {
	srv := NewMCPServer()
	defer srv.GetContainer().Close()

	err := srv.RunWithOptions(context.Background(), RunOptions{Transport: "sse"})
	if err == nil {
		t.Fatal("expected invalid transport error")
	}
	if !strings.Contains(err.Error(), "stdio or http") {
		t.Fatalf("error = %q, want stdio or http", err)
	}
}

func TestRunWithOptionsRejectsRemoteHTTPWithoutOptIn(t *testing.T) {
	srv := NewMCPServer()
	defer srv.GetContainer().Close()

	err := srv.RunWithOptions(context.Background(), RunOptions{
		Transport: HTTPTransport,
		HTTPAddr:  "192.168.1.20:8080",
	})
	if err == nil || !strings.Contains(err.Error(), "non-loopback") {
		t.Fatalf("error = %v, want non-loopback rejection", err)
	}
}

func TestHandleConfigureAppliesAllRuntimeSettings(t *testing.T) {
	srv := NewMCPServer()
	defer srv.GetContainer().Close()

	cacheSize := 12
	ttl := 45
	fileSize := 3
	depth := 7
	maxResults := 25
	extensions := []string{"go", ".py"}
	excluded := []string{"vendor"}
	_, _, err := srv.handleConfigure(context.Background(), nil, configureArgs{
		CacheMaxSizeMB:    &cacheSize,
		CacheTTLSeconds:   &ttl,
		MaxFileSizeMB:     &fileSize,
		AllowedExtensions: &extensions,
		ExcludedDirs:      &excluded,
		DefaultMaxDepth:   &depth,
		MaxResultsDefault: &maxResults,
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := srv.GetContainer().GetConfig()
	if cfg.Cache.MaxSizeMB != cacheSize || cfg.Cache.TTLSeconds != ttl ||
		cfg.Security.MaxFileSizeMB != fileSize || cfg.Language.DefaultMaxDepth != depth ||
		cfg.MaxResultsDefault != maxResults {
		t.Fatalf("configuration not fully applied: %#v", cfg)
	}
	if len(cfg.Security.AllowedExtensions) != 2 || len(cfg.Security.ExcludedDirs) != 1 {
		t.Fatalf("slice configuration not applied: %#v", cfg.Security)
	}
}

func TestHandleConfigureRejectsInvalidLimits(t *testing.T) {
	srv := NewMCPServer()
	defer srv.GetContainer().Close()

	invalid := 0
	if _, _, err := srv.handleConfigure(context.Background(), nil, configureArgs{CacheTTLSeconds: &invalid}); err == nil {
		t.Fatal("expected invalid cache TTL to be rejected")
	}
}
