package provider

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound  = errors.New("resource not found")
	ErrDenied    = errors.New("access denied")
	ErrNoPresign = errors.New("presigning is not allowed for this provider")
	// This is a special error that the provider can return to indicate that the object has not been modified since the specified time
	// It will be translated into a 304 Not Modified response by the server
	ErrNotModified = errors.New("resource not modified")
)

type ProviderType string

const (
	// ProviderTypeVoid is a provider that does nothing
	ProviderTypeVoid ProviderType = "void"
	// ProviderTypeLog is a provider that logs all operations
	ProviderTypeLog ProviderType = "log"
	// ProviderTypeS3 is a provider that uses AWS S3
	ProviderTypeS3 ProviderType = "s3"
	// ProviderTypeGocloud is a provider that uses the gocloud.dev CDK
	// This will support all the providers that gocloud supports including S3, Azure, Google Cloud, file and in-memory
	ProviderTypeGocloud ProviderType = "gocloud"
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

	GetObject(ctx context.Context, key string, opts GetOptions) (io.ReadCloser, ObjectInfo, error)
	PutObject(ctx context.Context, key string, data io.Reader, opts PutOptions) error
	GetTags(ctx context.Context, key string) (map[string]string, error)
	ListObjects(ctx context.Context, prefix string) (ListObjectsResponse, error)
}

type GetOptions struct {
	// If specified, the provider should only return the object if it has not been modified since this time, otherwise return ErrNotModified
	// If zero, the provider should return the object regardless of its modification time
	LastModified *time.Time
}

type PutOptions struct {
	// The content type of the object
	ContentType string

	ContentLength int64

	// The tags to apply to the object
	Tags map[string]string
}

// ObjectInfo contains metadata about an object
// The server will only act on the fields that are not nil
type ObjectInfo struct {
	// When the object was last modified
	LastModified *time.Time

	// The length of the object in bytes
	ContentLength *int64

	// The content type of the object
	ContentType *string
}

type ListObjectsResponse struct {
	// The keys of the objects found
	Keys []string
}

type PresignOperation string

const (
	PresignOperationDownload PresignOperation = "download"
	PresignOperationUpload   PresignOperation = "upload"
)

type Presigner interface {
	PresignURL(ctx context.Context, key string, direction PresignOperation) (string, error)
}
