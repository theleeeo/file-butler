// Description: This file contains the implementation of the client that will be used to interact with the server.
// The client will be used to upload, download and get tags of files.
package sdk

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
//		path: /inventory/vendors/logos
//		filename: huawei-main-x450.png
//		tags: map[string]string{"vendor": "huawei", "type": "main"}
//	  	data: []byte{...} // The content of the file
//
// The tags are the metadata that will be associated with the file. The data is the content of the file that will be uploaded.
// If the file already exists, it will be overwritten. If the file does not exist, it will be created.
func (c *Client) Upload(ctx context.Context, path string, filename string, tags map[string]string, data []byte) error {

	url, err := url.JoinPath(c.baseUrl, path, filename)
	if err != nil {
		return fmt.Errorf("cannot create path url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
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
