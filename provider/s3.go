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

func (s *S3Provider) GetObject(ctx context.Context, key string, opts GetOptions) (io.ReadCloser, ObjectInfo, error) {
	// Only return the object if it has been modified since the specified time
	// Otherwise return the ErrNotModified error
	if opts.LastModified != nil {
		headResp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: &s.bucketName,
			Key:    &key,
		})
		if err != nil {
			if strings.Contains(err.Error(), "NoSuchKey") {
				return nil, ObjectInfo{}, ErrNotFound
			}

			if strings.Contains(err.Error(), "AccessDenied") {
				return nil, ObjectInfo{}, ErrDenied
			}

			return nil, ObjectInfo{}, err
		}

		// If the object has not been modified since the specified time, return ErrNotModified which will be translated into a 304 Not Modified response by the server
		// It checks for Not After instead of Before because otherwise it will return false when the timestamps are equal
		if headResp.LastModified != nil && !headResp.LastModified.After(*opts.LastModified) {
			return nil, ObjectInfo{}, ErrNotModified
		}
	}

	getResp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucketName,
		Key:    &key,
	})
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") {
			return nil, ObjectInfo{}, ErrNotFound
		}

		if strings.Contains(err.Error(), "AccessDenied") {
			return nil, ObjectInfo{}, ErrDenied
		}

		return nil, ObjectInfo{}, err
	}

	return getResp.Body, ObjectInfo{LastModified: getResp.LastModified, ContentLength: getResp.ContentLength, ContentType: getResp.ContentType}, nil
}

func (s *S3Provider) PutObject(ctx context.Context, key string, data io.Reader, opts PutOptions) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucketName,
		Key:           &key,
		Body:          data,
		ContentLength: &opts.ContentLength,
		Tagging:       buildTagging(opts.Tags),
		ContentType:   &opts.ContentType,
	})
	if err != nil {
		return err
	}

	return nil
}

func buildTagging(tags map[string]string) *string {
	if len(tags) == 0 {
		return nil
	}

	var tagSet []string
	for k, v := range tags {
		tagSet = append(tagSet, fmt.Sprintf("%s=%s", k, v))
	}

	tagging := strings.Join(tagSet, "&")
	return &tagging
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

func (s *S3Provider) GetTags(ctx context.Context, key string) (map[string]string, error) {
	output, err := s.client.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
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

	tags := make(map[string]string, len(output.TagSet))
	for _, tag := range output.TagSet {
		tags[*tag.Key] = *tag.Value
	}

	return tags, nil
}

// Does currently not support pagination but the default max is 1000 so it is fine for now.
// Note for future developers: output.IsTruncated is a boolean that indicates if there are more objects to retrieve and output.NextContinuationToken is the token to use for the next request.
func (s *S3Provider) ListObjects(ctx context.Context, prefix string) (ListObjectsResponse, error) {
	output, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &s.bucketName,
		Prefix: &prefix,
	})
	if err != nil {
		return ListObjectsResponse{}, err
	}

	files := make([]string, len(output.Contents))
	for i, obj := range output.Contents {
		files[i] = *obj.Key

		// objects[i] = ObjectInfo{
		// 	Key:      *obj.Key,
		// 	Metadata: map[string]interface{}{},
		// }

		// if obj.Size != nil {
		// 	objects[i].Metadata["size"] = *obj.Size
		// }

		// if obj.LastModified != nil {
		// 	objects[i].Metadata["last_modified"] = *obj.LastModified
		// }
	}

	return ListObjectsResponse{
		Keys: files,
	}, nil
}

func (s *S3Provider) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucketName,
		Key:    &key,
	})
	if err != nil {
		// S3 seems to return a 200 OK even if the object does not exist so no need to check for that.
		return err
	}

	return nil
}
