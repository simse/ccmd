package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// only what we need for GetEntry
type S3API interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// abstracts manager.NewUploader’s Upload method
type Uploader interface {
	Upload(ctx context.Context, in *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

type S3Cache struct {
	URI      string
	Client   S3API    // for GetEntry
	Uploader Uploader // for PutEntry; tests inject, otherwise we build one
}

func (s *S3Cache) Validate() error {
	name := s.GetBucketName()

	// 1. Length 3–255
	if n := len(name); n < 3 || n > 255 {
		return fmt.Errorf("bucket name must be between 3 and 255 characters; got %d", n)
	}

	// 2. Only lowercase letters, numbers, periods, hyphens
	if m, _ := regexp.MatchString(`^[a-z0-9.-]+$`, name); !m {
		return errors.New("bucket name can contain only lowercase letters, numbers, periods, and hyphens")
	}

	// 3. Must begin and end with letter or number
	if !regexp.MustCompile(`^[a-z0-9]`).MatchString(name) ||
		!regexp.MustCompile(`[a-z0-9]$`).MatchString(name) {
		return errors.New("bucket name must begin and end with a letter or number")
	}

	// 4. No two adjacent periods
	if strings.Contains(name, "..") {
		return errors.New("bucket name must not contain consecutive periods")
	}

	// 5. Not formatted as IP address (e.g. 192.168.5.4)
	if strings.Count(name, ".") == 3 && net.ParseIP(name) != nil {
		return errors.New("bucket name must not be formatted as an IP address")
	}

	// 6. Forbidden prefixes
	forbiddenPrefixes := []string{"xn--", "sthree-", "amzn-s3-demo-"}
	for _, p := range forbiddenPrefixes {
		if strings.HasPrefix(name, p) {
			return fmt.Errorf("bucket name must not start with the reserved prefix %q", p)
		}
	}

	// 7. Forbidden suffixes
	forbiddenSuffixes := []string{"-s3alias", "--ol-s3", ".mrap", "--x-s3", "--table-s3"}
	for _, s := range forbiddenSuffixes {
		if strings.HasSuffix(name, s) {
			return fmt.Errorf("bucket name must not end with the reserved suffix %q", s)
		}
	}

	return nil
}

func (s *S3Cache) GetBucketName() string {
	return strings.Replace(s.URI, "s3://", "", 1)
}

func (s *S3Cache) getClient() (S3API, error) {
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
		Bucket: aws.String(s.GetBucketName()),
		Key:    aws.String(key),
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NoSuchBucket":
				return nil, fmt.Errorf("bucket %q does not exist", s.GetBucketName())
			case "NoSuchKey":
				return nil, fmt.Errorf("object %q not found in bucket %q", key, s.GetBucketName())
			default:
				return nil, fmt.Errorf("S3 API error %s: %s", apiErr.ErrorCode(), apiErr.ErrorMessage())
			}
		}

		return nil, err
	}

	return output.Body, nil
}

type CountingReader struct {
	Reader    io.Reader
	ByteCount int64
}

func (c *CountingReader) Read(p []byte) (int, error) {
	nn, err := c.Reader.Read(p)
	c.ByteCount += int64(nn)
	return nn, err
}

func (s *S3Cache) PutEntry(key string, body io.Reader) (int64, error) {
	up := s.Uploader
	if up == nil {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return 0, err
		}
		realClient := s3.NewFromConfig(cfg)
		up = manager.NewUploader(realClient)
	}

	counter := &CountingReader{Reader: body}
	if _, err := up.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.GetBucketName()),
		Key:    aws.String(key),
		Body:   counter,
	}); err != nil {
		return 0, err
	}

	return counter.ByteCount, nil
}
