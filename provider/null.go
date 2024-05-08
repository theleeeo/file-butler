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

type NullProvider struct{}

func (n *NullProvider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return NullCloser{strings.NewReader("null\n")}, nil
}

func (n *NullProvider) PutObject(ctx context.Context, key string, data io.Reader) error {
	return nil
}
