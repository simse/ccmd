package internal_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/simse/cmd-cache/internal"
)

// mock S3 API
type fakeS3 struct {
	out *s3.GetObjectOutput
	err error
}

func (f *fakeS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if aws.ToString(in.Bucket) != "mybucket" {
		return nil, fmt.Errorf("bucket=%q?", aws.ToString(in.Bucket))
	}
	if aws.ToString(in.Key) != "mykey" {
		return nil, fmt.Errorf("key=%q?", aws.ToString(in.Key))
	}
	return f.out, f.err
}

// fakeUploader implements our Uploader interface:
type fakeUploader struct {
	in  *s3.PutObjectInput
	err error
}

func (f *fakeUploader) Upload(_ context.Context, in *s3.PutObjectInput, _ ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	f.in = in
	// <— drain the body to advance the countingReader
	if _, err := io.Copy(io.Discard, in.Body); err != nil {
		return nil, err
	}
	return &manager.UploadOutput{}, f.err
}

// actual test cases
func TestGetBucketName(t *testing.T) {
	if b := (&internal.S3Cache{URI: "s3://abc"}).GetBucketName(); b != "abc" {
		t.Fatal("got", b)
	}
}

// func FuzzGetBucketName(f *testing.F) {
// 	// seed a couple of interesting cases
// 	f.Add("s3://bucket")
// 	f.Add("s3:///weird")
// 	f.Fuzz(func(t *testing.T, uri string) {
// 		c := &internal.S3Cache{URI: uri}
// 		bn := c.GetBucketName()
// 		// bucketName should never return a string containing "s3://"
// 		if strings.Contains(bn, "s3://") {
// 			t.Fatalf("bucketName(%q) = %q still contains prefix", uri, bn)
// 		}
// 	})
// }

func TestGetEntry(t *testing.T) {
	body := io.NopCloser(bytes.NewBufferString("hello"))
	cases := []struct {
		name     string
		apiOut   *s3.GetObjectOutput
		apiErr   error
		wantErr  string
		wantData string
	}{
		{"ok", &s3.GetObjectOutput{Body: body}, nil, "", "hello"},
		{"noBucket", nil, &smithy.GenericAPIError{Code: "NoSuchBucket"}, `bucket "mybucket" does not exist`, ""},
		{"noKey", nil, &smithy.GenericAPIError{Code: "NoSuchKey"}, `object "mykey" not found in bucket "mybucket"`, ""},
		{"otherAPI", nil, &smithy.GenericAPIError{Code: "Foo", Message: "bar"}, `S3 API error Foo: bar`, ""},
		{"genErr", nil, errors.New("boom"), `boom`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeS3{out: tc.apiOut, err: tc.apiErr}
			c := &internal.S3Cache{URI: "s3://mybucket", Client: f}

			rc, err := c.GetEntry("mykey")
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("got=%v, want=%v", err, tc.wantErr)
				}
				return
			}

			data, _ := io.ReadAll(rc)
			if string(data) != tc.wantData {
				t.Errorf("data=%q; want %q", data, tc.wantData)
			}
		})
	}

	// check that nothing breaks when S3 returns an empty body
	f := &fakeS3{out: &s3.GetObjectOutput{Body: nil}, err: nil}
	c := &internal.S3Cache{URI: "s3://test", Client: f}

	_, err := c.GetEntry("foobar")
	if err == nil {
		t.Errorf("empty body didn't cause an error")
	}
}

func TestPutEntry_WithManager(t *testing.T) {
	// inject the fakeUploader so we never hit real AWS
	fu := &fakeUploader{}
	c := &internal.S3Cache{
		URI:      "s3://mybucket",
		Uploader: fu,
	}

	// successful upload
	n, err := c.PutEntry("mykey", bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("bytes = %d; want 5", n)
	}
	if aws.ToString(fu.in.Bucket) != "mybucket" || aws.ToString(fu.in.Key) != "mykey" {
		t.Errorf("wrong bucket/key: %v", fu.in)
	}

	// simulate an upload error
	fu.err = errors.New("uh-oh")
	n, err = c.PutEntry("foo", bytes.NewBufferString("x"))
	if err == nil || err.Error() != "uh-oh" {
		t.Fatalf("expected upload error, got %v", err)
	}
	if n != 0 {
		t.Errorf("on error, count should be 0, got %d", n)
	}
}

func TestCountingReader(t *testing.T) {
	t.Run("successive reads accumulate ByteCount", func(t *testing.T) {
		data := []byte("abcdef")
		cr := &internal.CountingReader{Reader: bytes.NewBuffer(data)}

		// first read: 3 bytes
		buf := make([]byte, 3)
		n, err := cr.Read(buf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if n != 3 {
			t.Errorf("got %d bytes, want 3", n)
		}
		if cr.ByteCount != 3 {
			t.Errorf("ByteCount = %d, want 3", cr.ByteCount)
		}

		// second read: remaining 3 bytes
		buf2 := make([]byte, 10)
		n2, err2 := cr.Read(buf2)
		if err2 != io.EOF && err2 != nil {
			t.Fatalf("expected EOF or nil, got %v", err2)
		}
		if n2 != 3 {
			t.Errorf("got %d bytes, want 3", n2)
		}
		if cr.ByteCount != 6 {
			t.Errorf("ByteCount = %d, want 6", cr.ByteCount)
		}
	})

	t.Run("underlying Read error does not change ByteCount", func(t *testing.T) {
		readErr := errors.New("boom")
		cr := &internal.CountingReader{Reader: errReader{err: readErr}}

		buf := make([]byte, 5)
		n, err := cr.Read(buf)
		if err != readErr {
			t.Errorf("error = %v, want %v", err, readErr)
		}
		if n != 0 {
			t.Errorf("got %d bytes, want 0", n)
		}
		if cr.ByteCount != 0 {
			t.Errorf("ByteCount = %d, want 0", cr.ByteCount)
		}
	})
}

// errReader always returns a fixed error.
type errReader struct{ err error }

func (e errReader) Read(p []byte) (int, error) {
	return 0, e.err
}

func FuzzCountingReader(f *testing.F) {
	// seed with some data sizes
	f.Add([]byte("hello"))
	f.Add([]byte{})
	long := make([]byte, 1024)
	f.Add(long)

	f.Fuzz(func(t *testing.T, data []byte) {
		cr := &internal.CountingReader{Reader: bytes.NewReader(data)}
		buf := make([]byte, len(data))
		n, err := cr.Read(buf)
		if err != nil && err != io.EOF {
			t.Fatalf("unexpected err: %v", err)
		}
		// cr.n must equal the number of bytes actually read
		if int64(n) != cr.ByteCount {
			t.Errorf("countingReader counted %d but Read returned %d", cr.ByteCount, n)
		}
	})
}
