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
