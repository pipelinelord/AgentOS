package memory

import (
	"context"
	"fmt"
)

type ChromaDriver struct {
}

func NewChromaDriver(ctx context.Context, collectionName string) (*ChromaDriver, error) {
	// The original PoC code used an older chroma-go API that is incompatible with v0.4.1.
	// Since we are using LocalMemoryDriver as a fallback, we stub this out for now.
	return nil, fmt.Errorf("ChromaDriver requires update for chroma-go v0.4.1 API")
}

func (d *ChromaDriver) Write(ctx context.Context, key, val string) error {
	return fmt.Errorf("not implemented")
}

func (d *ChromaDriver) Read(ctx context.Context, query string) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}
