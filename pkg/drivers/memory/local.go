package memory

import (
	"context"
	"strings"
	"sync"
)

type LocalMemoryDriver struct {
	mu   sync.RWMutex
	data map[string]string // key -> doc
}

func NewLocalMemoryDriver() *LocalMemoryDriver {
	return &LocalMemoryDriver{
		data: make(map[string]string),
	}
}

func (d *LocalMemoryDriver) Write(ctx context.Context, key, val string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.data[key] = val
	return nil
}

func (d *LocalMemoryDriver) Read(ctx context.Context, query string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []string
	queryLower := strings.ToLower(query)
	
	for _, val := range d.data {
		// Basic substring match as fallback for vector search
		if strings.Contains(strings.ToLower(val), queryLower) {
			results = append(results, val)
		}
	}
	
	return results, nil
}
