package provider

import (
	"context"
	"errors"
	"io"
)

var (
	ErrNotFound  = errors.New("resource not found")
	ErrDenied    = errors.New("access denied")
	ErrNoPresign = errors.New("presigning is not allowed for this provider")
)

type ProviderType string

const (
	// ProviderTypeVoid is a provider that does nothing
	ProviderTypeVoid ProviderType = "void"
	// ProviderTypeLog is a provider that logs all operations
	ProviderTypeLog ProviderType = "log"
	// ProviderTypeS3 is a provider that uses AWS S3
	ProviderTypeS3 ProviderType = "s3"
)

type Config interface {
	Id() string
}

type ConfigBase struct {
	ID         string
	AuthPlugin string `json:"auth-plugin"`
}

func (c *ConfigBase) Id() string {
	return c.ID
}

type Provider interface {
	// Id returns the provider ID which must be unique among all providers
	Id() string
	// AuthPlugin optionally returns the name of the auth plugin that should be used for this provider
	// If the provider does not specify an auth plugin the default one will be used
	AuthPlugin() string

	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key string, data io.Reader, length int64) error
}

type PresignOperation string

const (
	PresignOperationDownload PresignOperation = "download"
	PresignOperationUpload   PresignOperation = "upload"
)

type Presigner interface {
	PresignURL(ctx context.Context, key string, direction PresignOperation) (string, error)
}
