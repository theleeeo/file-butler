package provider

import (
	"context"
	"io"
	"strings"
)

type VoidConfig struct {
	ConfigBase
}

func NewVoidProvider(cfg *VoidConfig) *VoidProvider {
	return &VoidProvider{
		id:         cfg.ID,
		authPlugin: cfg.AuthPlugin,
	}
}

type VoidProvider struct {
	id         string
	authPlugin string
}

func (n *VoidProvider) Id() string {
	return n.id
}

func (n *VoidProvider) AuthPlugin() string {
	return n.authPlugin
}

func (n *VoidProvider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("null\n")), nil
}

func (n *VoidProvider) PutObject(ctx context.Context, key string, data io.Reader, length int64, tags map[string]string) error {
	return nil
}

func (n *VoidProvider) GetTags(ctx context.Context, key string) (map[string]string, error) {
	return nil, nil
}
