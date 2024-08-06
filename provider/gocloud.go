package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/memblob"
	_ "gocloud.dev/blob/s3blob"
	"gocloud.dev/gcerrors"
)

type GocloudConfig struct {
	ConfigBase
	DriverURL string `json:"driver-url"`
}

func NewGocloudProvider(cfg *GocloudConfig) (*GocloudProvider, error) {
	if cfg.DriverURL == "" {
		return nil, fmt.Errorf("driverURL is required")
	}

	bucket, err := blob.OpenBucket(context.Background(), cfg.DriverURL)
	if err != nil {
		return nil, fmt.Errorf("could not open bucket: %w", err)
	}

	// defer bucket.Close()

	return &GocloudProvider{
		id:         cfg.ID,
		authPlugin: cfg.AuthPlugin,
		bucket:     bucket,
	}, nil
}

type GocloudProvider struct {
	id         string
	authPlugin string

	bucket *blob.Bucket
}

func (n *GocloudProvider) Id() string {
	return n.id
}

func (n *GocloudProvider) AuthPlugin() string {
	return n.authPlugin
}

func (n *GocloudProvider) GetObject(ctx context.Context, key string, opts GetOptions) (io.ReadCloser, ObjectInfo, error) {
	r, err := n.bucket.NewReader(ctx, key, nil)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, err
	}

	contentSize := r.Size()
	lastMod := r.ModTime()
	return r, ObjectInfo{
		ContentLength: &contentSize,
		LastModified:  &lastMod,
	}, nil
}

func (n *GocloudProvider) PutObject(ctx context.Context, key string, data io.Reader, opts PutOptions) error {
	writeCtx, cancelWrite := context.WithCancel(ctx)
	defer cancelWrite()

	w, err := n.bucket.NewWriter(writeCtx, key, nil)
	if err != nil {
		return err
	}

	if _, err := io.Copy(w, data); err != nil {
		cancelWrite()
		w.Close()
		return err
	}

	w.Close()
	return nil
}

func (n *GocloudProvider) GetTags(ctx context.Context, key string) (map[string]string, error) {
	log.Println("GetTags not implemented for gocloud provider")
	return nil, nil
}

func (n *GocloudProvider) ListObjects(ctx context.Context, prefix string) ([]ListObject, error) {
	iter := n.bucket.List(&blob.ListOptions{
		Prefix: prefix,
	})

	var objects []ListObject
	for {
		obj, err := iter.Next(ctx)

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		objects = append(objects, ListObject{
			LastModified: &obj.ModTime,
			Size:         &obj.Size,
			Key:          obj.Key,
			Prefix:       prefix,
		})
	}

	return objects, nil
}
