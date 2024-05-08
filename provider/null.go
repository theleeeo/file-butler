package provider

import (
	"context"
	"io"
	"strings"
)

type NullCloser struct {
	io.Reader
}

func (n NullCloser) Close() error { return nil }

type NullConfig struct {
	ID string
}

func (n *NullConfig) Id() string {
	return n.ID
}

func NewNullProvider(cfg *NullConfig) *NullProvider {
	return &NullProvider{
		id: cfg.ID,
	}
}

type NullProvider struct {
	id string
}

func (n *NullProvider) Id() string {
	return n.id
}

func (n *NullProvider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return NullCloser{strings.NewReader("null\n")}, nil
}

func (n *NullProvider) PutObject(ctx context.Context, key string, data io.Reader) error {
	return nil
}
