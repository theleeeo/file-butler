package provider

import (
	"context"
	"io"
	"strings"
)

// NullCloser is a ReadCloser that does nothing when closed
// It is used to wrap a Reader that does not need to be closed
type NullCloser struct {
	io.Reader
}

func (n NullCloser) Close() error { return nil }

type VoidConfig struct {
	ID string
}

func (n *VoidConfig) Id() string {
	return n.ID
}

func NewVoidProvider(cfg *VoidConfig) *VoidProvider {
	return &VoidProvider{
		id: cfg.ID,
	}
}

type VoidProvider struct {
	id string
}

func (n *VoidProvider) Id() string {
	return n.id
}

func (n *VoidProvider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return NullCloser{strings.NewReader("null\n")}, nil
}

func (n *VoidProvider) PutObject(ctx context.Context, key string, data io.Reader) error {
	return nil
}
