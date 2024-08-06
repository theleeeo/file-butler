package provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
)

type LogConfig struct {
	ConfigBase
}

func NewLogProvider(cfg *LogConfig) *LogProvider {
	return &LogProvider{
		id:         cfg.ID,
		authPlugin: cfg.AuthPlugin,
	}
}

type LogProvider struct {
	id         string
	authPlugin string
}

func (n *LogProvider) Id() string {
	return n.id
}

func (n *LogProvider) AuthPlugin() string {
	return n.authPlugin
}

func (n *LogProvider) GetObject(ctx context.Context, key string, opts GetOptions) (io.ReadCloser, ObjectInfo, error) {
	log.Printf("GetObject %s, opts=%v\n", key, opts)
	return io.NopCloser(strings.NewReader("Hello World!\n")), ObjectInfo{}, nil
}

func (n *LogProvider) PutObject(ctx context.Context, key string, data io.Reader, opts PutOptions) error {
	var l string
	if opts.ContentLength < 1024 {
		l = fmt.Sprintf("%db", opts.ContentLength)
	} else if opts.ContentLength < 1024*1024 {
		l = fmt.Sprintf("%dkb", opts.ContentLength/1024)
	} else {
		l = fmt.Sprintf("%dmb", opts.ContentLength/1024/1024)
	}

	log.Printf("PutObject key=%s, size=%s\n, tags=%v", key, l, opts.Tags)
	return nil
}

func (n *LogProvider) GetTags(ctx context.Context, key string) (map[string]string, error) {
	log.Printf("GetTags %s\n", key)
	return nil, nil
}

func (n *LogProvider) ListObjects(ctx context.Context, prefix string) ([]ListObject, error) {
	log.Printf("ListObjects %s\n", prefix)
	return nil, nil
}
