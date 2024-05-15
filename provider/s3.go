package provider

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var _ Provider = &S3Provider{}
var _ Presigner = &S3Provider{}

type S3Config struct {
	ConfigBase
	Bucket         string
	Region         string
	Profile        string
	PresignEnabled bool `json:"presign-enabled"`
}

func NewS3Provider(cfg *S3Config) (*S3Provider, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region), config.WithSharedConfigProfile(cfg.Profile))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}

	client := s3.NewFromConfig(clientCfg)

	var presignClient *s3.PresignClient
	if cfg.PresignEnabled {
		presignClient = s3.NewPresignClient(client)
	}

	return &S3Provider{
		id:            cfg.ID,
		authPlugin:    cfg.AuthPlugin,
		bucketName:    cfg.Bucket,
		client:        client,
		presignClient: presignClient,
	}, nil
}

type S3Provider struct {
	id         string
	authPlugin string
	bucketName string

	client        *s3.Client
	presignClient *s3.PresignClient
}

func (s *S3Provider) Id() string {
	return s.id
}

func (s *S3Provider) AuthPlugin() string {
	return s.authPlugin
}

func (s *S3Provider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucketName,
		Key:    &key,
	})
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") {
			return nil, ErrNotFound
		}

		if strings.Contains(err.Error(), "AccessDenied") {
			return nil, ErrDenied
		}

		return nil, err
	}

	return output.Body, nil
}

func (s *S3Provider) PutObject(ctx context.Context, key string, data io.Reader, length int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucketName,
		Key:           &key,
		Body:          data,
		ContentLength: &length,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *S3Provider) PresignURL(ctx context.Context, key string, op PresignOperation) (string, error) {
	if s.presignClient == nil {
		return "", ErrNoPresign
	}

	var req *v4.PresignedHTTPRequest
	var err error

	if op == PresignOperationDownload {
		req, err = s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: &s.bucketName,
			Key:    &key,
		})
	} else if op == PresignOperationUpload {
		req, err = s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: &s.bucketName,
			Key:    &key,
		})
	} else {
		return "", fmt.Errorf("unsupported presign operation: %s", op)
	}

	if err != nil {
		return "", err
	}

	return req.URL, nil
}

// func (s *S3Provider) ListObjects() ([]ObjectInfo, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	output, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
// 		Bucket: &s.bucketName,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	objects := make([]ObjectInfo, len(output.Contents))
// 	for i, obj := range output.Contents {
// 		objects[i] = ObjectInfo{
// 			Key:      *obj.Key,
// 			Metadata: map[string]interface{}{},
// 		}

// 		if obj.Size != nil {
// 			objects[i].Metadata["size"] = *obj.Size
// 		}

// 		if obj.LastModified != nil {
// 			objects[i].Metadata["last_modified"] = *obj.LastModified
// 		}
// 	}

// 	return objects, nil
// }
