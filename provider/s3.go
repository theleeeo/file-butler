package provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var _ Provider = &S3Provider{}

type S3Config struct {
	ID      string
	Bucket  string
	Region  string
	Profile string
}

func (n *S3Config) Id() string {
	return n.ID
}

func NewS3Provider(cfg *S3Config) (*S3Provider, error) {
	fmt.Println(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region), config.WithSharedConfigProfile(cfg.Profile))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	client := s3.NewFromConfig(clientCfg)

	return &S3Provider{
		bucketName: cfg.Bucket,
		client:     client,
	}, nil
}

type S3Provider struct {
	bucketName string

	client *s3.Client
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

func (s *S3Provider) PutObject(ctx context.Context, key string, data io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucketName,
		Key:    &key,
		Body:   data,
	})
	if err != nil {
		return err
	}

	return nil
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
