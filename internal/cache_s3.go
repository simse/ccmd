package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type S3Cache struct {
	URI    string
	Client *s3.Client
}

func (s *S3Cache) bucketName() string {
	return strings.Replace(s.URI, "s3://", "", 1)
}

func (s *S3Cache) getClient() (*s3.Client, error) {
	if s.Client == nil {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return nil, err
		}

		s.Client = s3.NewFromConfig(cfg)
	}

	return s.Client, nil
}

func (s *S3Cache) GetFriendlyName() string {
	return "s3"
}

func (s *S3Cache) GetEntry(key string) (io.ReadCloser, error) {
	client, err := s.getClient()

	if err != nil {
		return nil, err
	}

	// check if entry exists by getting it
	output, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName()),
		Key:    aws.String(key),
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NoSuchBucket":
				return nil, fmt.Errorf("bucket %q does not exist", s.bucketName())
			case "NoSuchKey":
				return nil, fmt.Errorf("object %q not found in bucket %q", key, s.bucketName())
			default:
				return nil, fmt.Errorf("S3 API error %s: %s", apiErr.ErrorCode(), apiErr.ErrorMessage())
			}
		}
	}

	return output.Body, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	nn, err := c.r.Read(p)
	c.n += int64(nn)
	return nn, err
}

func (s *S3Cache) PutEntry(key string, contents io.Reader) (int64, error) {
	client, err := s.getClient()
	if err != nil {
		return 0, err
	}

	counter := &countingReader{r: contents}
	uploader := manager.NewUploader(client)
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName()),
		Key:    aws.String(key),
		Body:   counter,
	})
	if err != nil {
		return 0, err
	}

	return counter.n, nil
}
