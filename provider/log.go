package provider

import (
	"context"
	"io"
	"log"
	"strings"
)

type LogConfig struct {
	ID string
}

func (n *LogConfig) Id() string {
	return n.ID
}

type LogProvider struct {
}

func (n *LogProvider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	log.Printf("GetObject %s", key)
	return NullCloser{strings.NewReader("Hello World!\n")}, nil
}

func (n *LogProvider) PutObject(ctx context.Context, key string, data io.Reader) error {
	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	log.Printf("PutObject key=%s, len=%dmb\n", key, len(b)/1000000)
	return nil
}
