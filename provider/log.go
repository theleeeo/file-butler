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

func (n *LogProvider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	log.Printf("GetObject %s", key)
	return io.NopCloser(strings.NewReader("Hello World!\n")), nil
}

func (n *LogProvider) PutObject(ctx context.Context, key string, data io.Reader, length int64) error {
	var l string
	if length < 1024 {
		l = fmt.Sprintf("%db", length)
	} else if length < 1024*1024 {
		l = fmt.Sprintf("%dkb", length/1024)
	} else {
		l = fmt.Sprintf("%dmb", length/1024/1024)
	}

	log.Printf("PutObject key=%s, size=%s\n", key, l)
	return nil
}
