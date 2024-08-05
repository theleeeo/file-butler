package sdk

import (
	"net/http"
	"time"
)

type Client struct {
	baseUrl string
	client  *http.Client
}

func New(baseUrl string, timeout time.Duration) *Client {

	if baseUrl == "" {
		baseUrl = "http://localhost:8080"
	}

	client := &http.Client{
		Timeout: timeout,
	}

	return &Client{
		baseUrl: baseUrl,
		client:  client,
	}
}

func (c *Client) Upload(path string, filename string, tags map[string]string) {
	// upload file
}

func (c *Client) Download() {
	// download file
}

func (c *Client) GetTags() {
}
