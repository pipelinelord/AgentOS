package io

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalFSDriver struct {
	baseDir string
}

func NewLocalFSDriver(baseDir string) (*LocalFSDriver, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	// Ensure base dir exists
	if err := os.MkdirAll(absBase, 0755); err != nil {
		return nil, err
	}
	return &LocalFSDriver{baseDir: absBase}, nil
}

func (d *LocalFSDriver) resolvePath(reqPath string) (string, error) {
	// Treat the request path as relative to the base directory if it starts with /
	// Strip leading slashes to make it cleanly relative.
	cleanReq := filepath.Clean(strings.TrimPrefix(reqPath, "/"))
	if cleanReq == ".." || strings.HasPrefix(cleanReq, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("access denied: path escapes sandbox")
	}

	fullPath := filepath.Join(d.baseDir, cleanReq)

	// Final security check
	if !strings.HasPrefix(fullPath, d.baseDir) {
		return "", fmt.Errorf("access denied: path escapes sandbox")
	}

	return fullPath, nil
}

func (d *LocalFSDriver) Read(ctx context.Context, path string) (string, error) {
	fullPath, err := d.resolvePath(path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (d *LocalFSDriver) Write(ctx context.Context, path, content string) error {
	fullPath, err := d.resolvePath(path)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}
