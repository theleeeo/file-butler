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

func (p *Provider) GetObject(ctx context.Context, key string, opts provider.GetOptions) (io.ReadCloser, provider.ObjectInfo, error) {
	called := p.Called(ctx, key, opts)

	r1 := called.Get(0)
	if r1 == nil {
		return nil, provider.ObjectInfo{}, called.Error(2)
	}

	var rc io.ReadCloser
	if r1, ok := r1.(io.ReadCloser); ok {
		rc = r1
	}

	if r1, ok := r1.(io.Reader); ok {
		rc = io.NopCloser(r1)
	}

	if r1, ok := r1.(string); ok {
		rc = io.NopCloser(strings.NewReader(r1))
	}

	if r1, ok := r1.([]byte); ok {
		rc = io.NopCloser(bytes.NewBuffer(r1))
	}

	return rc, called.Get(1).(provider.ObjectInfo), nil
}

func (p *Provider) PutObject(ctx context.Context, key string, data io.Reader, length int64, tags map[string]string) error {
	completeData, err := io.ReadAll(data)
	if err != nil {
		panic(err)
	}

	called := p.Called(ctx, key, completeData, length, tags)
	return called.Error(0)
}

func (p *Provider) GetTags(ctx context.Context, key string) (map[string]string, error) {
	called := p.Called(ctx, key)
	if called.Get(0) == nil {
		return nil, called.Error(1)
	}

	return called.Get(0).(map[string]string), called.Error(1)
}
