package mocks

import (
	"context"

	"github.com/theleeeo/file-butler/provider"
)

var _ provider.Provider = (*PresignProvider)(nil)
var _ provider.Presigner = (*PresignProvider)(nil)

func NewPresignProvider(cfg provider.ConfigBase) *PresignProvider {
	return &PresignProvider{
		Provider: Provider{
			cfg: cfg,
		},
	}
}

type PresignProvider struct {
	Provider
}

func (p *PresignProvider) PresignURL(ctx context.Context, key string, direction provider.PresignOperation) (string, error) {
	called := p.Called(ctx, key, direction)
	if called.Get(0) == nil {
		return "", called.Error(1)
	}

	return called.String(0), called.Error(1)
}
