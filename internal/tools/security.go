package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var securityLimits = struct {
	sync.RWMutex
	maxFileSizeBytes int64
	allowedExts      map[string]bool
}{
	maxFileSizeBytes: 5 * 1024 * 1024,
}

// SetSecurityLimits updates process-wide file limits used by tool operations.
func SetSecurityLimits(maxFileSizeMB int, allowedExtensions []string) {
	securityLimits.Lock()
	defer securityLimits.Unlock()

	if maxFileSizeMB > 0 {
		securityLimits.maxFileSizeBytes = int64(maxFileSizeMB) * 1024 * 1024
	}

	if len(allowedExtensions) == 0 {
		securityLimits.allowedExts = nil
		return
	}

	exts := make(map[string]bool, len(allowedExtensions))
	for _, ext := range allowedExtensions {
		ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
		if ext != "" {
			exts[ext] = true
		}
	}
	securityLimits.allowedExts = exts
}

func checkFileAllowed(path string) error {
	securityLimits.RLock()
	maxBytes := securityLimits.maxFileSizeBytes
	allowedExts := securityLimits.allowedExts
	securityLimits.RUnlock()

	if len(allowedExts) > 0 {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		if !allowedExts[ext] {
			return fmt.Errorf("file extension .%s is not allowed", ext)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", path)
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return fmt.Errorf("file %s exceeds max_file_size_mb", path)
	}
	return nil
}

func isAllowedRegularFile(path string, info os.FileInfo) bool {
	if info == nil || info.IsDir() {
		return false
	}
	if err := checkFileAllowed(path); err != nil {
		return false
	}
	return true
}
