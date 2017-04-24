package provider

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"

	minio "github.com/minio/minio-go"
	"github.com/pkg/errors"
)

// NewS3 returns an S3 client capable of providing files from any S3 compatible
// service such as Minio or Amazon S3 itself.
func NewS3(bucket string) (*S3, error) {
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKeyID := os.Getenv("S3_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("S3_ACCESS_KEY_SECRET")
	useSSL := os.Getenv("S3_DONT_USE_SSL") != "TRUE"

	// Initialize minio client object.
	client, err := minio.New(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create the s3 client")
	}

	return &S3{
		bucket: bucket,
		client: client,
	}, nil
}

// S3 is a file provider that is capable of providing files from any S3
// compatible service such as Minio or Amazon S3 itself.
type S3 struct {
	bucket string
	client *minio.Client
}

// Provide loads the file from the S3 client.
func (s *S3) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {

	// Get the reader from the minio client.
	r, err := s.client.GetObject(s.bucket, filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Read the image into the buffer. You would think that we could just return
	// the reader returned by the client, but we won't actually get the right
	// error when it gets returned, so we need to read in the file, and then pass
	// it down in a compatible interface.
	buf := bytes.NewBuffer(nil)

	// Copy the file reader into the bytes buffer, and if there's an error, check
	// to see if it was a "not found" error, and return as such if that's the
	// case.
	if _, err := io.Copy(buf, r); err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNotFound
		}

		return nil, err
	}

	// The bytes buffer isn't a closer by nature, just wrap it with a no-op closer
	// to satisfy the interface.
	return ioutil.NopCloser(buf), nil
}
