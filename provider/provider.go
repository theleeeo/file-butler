package provider

import (
	"context"
	"errors"
	"io"
)

var (
	ErrNotFound = errors.New("resource not found")
	ErrDenied   = errors.New("access denied")
)

type ProviderType string

const (
	// ProviderTypeNull is a provider that does nothing
	ProviderTypeNull ProviderType = "null"
	// ProviderTypeLog is a provider that logs all operations
	ProviderTypeLog ProviderType = "log"
	// ProviderTypeS3 is a provider that uses AWS S3
	ProviderTypeS3 ProviderType = "s3"
)

type Config interface {
	Id() string
}

type Provider interface {
	Id() string
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key string, data io.Reader) error
}
