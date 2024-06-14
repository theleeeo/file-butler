package provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

func uploadFileToAzureBlob(ctx context.Context, accountName, accountKey, containerName, filePath, filename string) error {
	// Create a default request pipeline using your storage account name and account key.
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)
	serviceClient, err := azblob.NewClientWithSharedKeyCredential(serviceURL, credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create service client: %w", err)
	}

	containerClient := serviceClient.ServiceClient().NewContainerClient(containerName)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	blobClient := containerClient.NewBlockBlobClient(filename)

	// Upload the file
	_, err = blobClient.UploadStream(ctx, file, &blockblob.UploadStreamOptions{})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

type AzureConfig struct {
	ConfigBase

	AccountName string `json:"account-name"`
	AccountKey  string `json:"account-key"`
	Container   string `json:"container"`
}

func NewAzureProvider(cfg *AzureConfig) (*AzureProvider, error) {
	credential, err := azblob.NewSharedKeyCredential(cfg.AccountName, cfg.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.AccountName)
	serviceClient, err := azblob.NewClientWithSharedKeyCredential(serviceURL, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create service client: %w", err)
	}

	containerClient := serviceClient.ServiceClient().NewContainerClient(cfg.Container)

	return &AzureProvider{
		id:              cfg.ID,
		authPlugin:      cfg.AuthPlugin,
		containerClient: containerClient,
	}, nil
}

type AzureProvider struct {
	id         string
	authPlugin string

	containerClient *container.Client
}

func (n *AzureProvider) Id() string {
	return n.id
}

func (n *AzureProvider) AuthPlugin() string {
	return n.authPlugin
}

func (n *AzureProvider) GetObject(ctx context.Context, key string, opts GetOptions) (io.ReadCloser, ObjectInfo, error) {
	return nil, ObjectInfo{}, nil
}

func (n *AzureProvider) PutObject(ctx context.Context, key string, data io.Reader, opts PutOptions) error {
	blobClient := n.containerClient.NewBlockBlobClient(key)

	_, err := blobClient.UploadStream(ctx, data, &blockblob.UploadStreamOptions{})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

func (n *AzureProvider) GetTags(ctx context.Context, key string) (map[string]string, error) {
	log.Println("GetTags not implemented for gocloud provider")
	return nil, nil
}
