package mocks

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/stretchr/testify/mock"
	"github.com/theleeeo/file-butler/provider"
)

var _ provider.Provider = (*Provider)(nil)

func NewProvider(cfg provider.ConfigBase) *Provider {
	return &Provider{
		cfg: cfg,
	}
}

type Provider struct {
	mock.Mock

	cfg provider.ConfigBase
}

func (p *Provider) AuthPlugin() string {
	return p.cfg.AuthPlugin
}

func (p *Provider) Id() string {
	return p.cfg.ID
}

func (p *Provider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	called := p.Called(ctx, key)

	r1 := called.Get(0)
	if r1 == nil {
		return nil, called.Error(1)
	}

	if r1, ok := r1.(io.ReadCloser); ok {
		return r1, called.Error(1)
	}

	if r1, ok := r1.(io.Reader); ok {
		return io.NopCloser(r1), called.Error(1)
	}

	if r1, ok := r1.(string); ok {
		return io.NopCloser(strings.NewReader(r1)), called.Error(1)
	}

	if r1, ok := r1.([]byte); ok {
		return io.NopCloser(bytes.NewBuffer(r1)), called.Error(1)
	}

	return r1.(io.ReadCloser), called.Error(1)
}

func (p *Provider) PutObject(ctx context.Context, key string, data io.Reader) error {
	completeData, err := io.ReadAll(data)
	if err != nil {
		panic(err)
	}

	called := p.Called(ctx, key, completeData)
	return called.Error(0)
}
