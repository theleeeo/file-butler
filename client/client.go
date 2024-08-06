// Description: This file contains the implementation of the client that will be used to interact with the server.
// The client will be used to upload, download and get tags of files.
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseUrl string
	client  *http.Client
}

// New creates a new client with the provided base url and timeout. If the base url is empty, it will default to "http://localhost:8080".
// If the provided url is not a valid url, an error will be returned. The timeout is used to set the timeout for the http client. If the timeout is 0, it will default to 10 seconds.
func New(baseUrl string, timeout time.Duration) (*Client, error) {

	if baseUrl == "" {
		baseUrl = "http://localhost:8080"
	}

	_, err := url.Parse(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("the provided url is not a valid url: %w", err)
	}

	if timeout.Seconds() == 0 {
		timeout = 10 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
	}

	return &Client{
		baseUrl: baseUrl,
		client:  client,
	}, nil
}

// Upload uploads a file to the server. The path is the path to the file that will be uploaded. The filename is the name of the file that will be uploaded.
// For example:
//
//   - data: []byte{...}
//   - path: "/inventory/vendors/logos"
//   - filename: "huawei-main-x450.png"
//   - tags: map[string]string{"vendor": "huawei", "type": "main"}
//
// The tags are the metadata that will be associated with the file. The data is the content of the file that will be uploaded.
// If the file already exists, it will be overwritten. If the file does not exist, it will be created.
func (c *Client) Upload(ctx context.Context, data []byte, path string, filename string, tags map[string]string) error {

	reqUrl, err := url.JoinPath(c.baseUrl, path, filename)
	if err != nil {
		return fmt.Errorf("cannot create path url: %w", err)
	}

	if len(tags) > 0 {
		values := url.Values{}
		// set tags
		for k, v := range tags {
			values.Set(k, v)
		}
		reqUrl = reqUrl + "?" + values.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqUrl, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("could not upload file: %s", string(respBody))
	}

	return nil
}

// Download downloads a file from the server. The path is the path to the file that will be downloaded. The filename is the name of the file that will be downloaded.
func (c *Client) Download(filePath string) ([]byte, error) {
	return nil, nil
}

// GetTags gets the tags of a file from the server. The path is the path to the file that the tags will be retrieved from.
// The tags are returned as a map of string to string. If the file does not exist, an error will be returned.
func (c *Client) GetTags(filePath string) (map[string]string, error) {
	return nil, nil
}

type Option struct {
	baseUrl string
	timeout time.Duration
	tags    map[string]string
	ctx     context.Context
}

type optionFunc func(*Option)

func WithBaseUrl(baseUrl string) optionFunc {
	return func(o *Option) {
		o.baseUrl = baseUrl
	}
}

func WithTimeout(timeout time.Duration) optionFunc {
	return func(o *Option) {
		o.timeout = timeout
	}
}

func WithTags(tags map[string]string) optionFunc {
	return func(o *Option) {
		o.tags = tags
	}
}

func WithContext(ctx context.Context) optionFunc {
	return func(o *Option) {
		o.ctx = ctx
	}
}

// Quick upload that creates the client and uploads the file in one go.
// use with Options to set the base url, timeout, tags and context.
// e.g
//
//	Upload("/folder/to/store/in", "filename.jpg", []byte("imageDataBytes"),
//		WithBaseUrl("http://localhost:8080"),
//		WithTimeout(10*time.Second),
//		WithTags(map[string]string{"tag1": "tag1value", "tag2": "tag2value"}),
//	 )
func Upload(path string, filename string, data []byte, opts ...optionFunc) error {
	o := &Option{
		baseUrl: "http://localhost:8080",
		timeout: 10 * time.Second,
	}
	for _, opt := range opts {
		opt(o)
	}

	client, err := New(o.baseUrl, o.timeout)
	if err != nil {
		return err
	}
	if o.ctx == nil {
		o.ctx = context.Background()
	}

	return client.Upload(o.ctx, data, path, filename, o.tags)
}
