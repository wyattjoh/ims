package provider

import (
	"context"
	"io"

	"github.com/pkg/errors"

	"google.golang.org/cloud/storage"
)

// NewGCS will create the GCS Provider.
func NewGCS(ctx context.Context, bucket string) (*GCS, error) {

	// Creates a storage client for the span of this request.
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create storage client")
	}

	return &GCS{
		bucket: client.Bucket(bucket),
	}, nil
}

// GCS provides a way to access files from Google Cloud Storage. Credentials
// used are loaded from the `GOOGLE_APPLICATION_CREDENTIALS` environment
// variable.
type GCS struct {
	bucket *storage.BucketHandle
}

// Provide provides a file by making a request to Google Cloud Storage with the
// specified key and then returning the response body when the request was
// complete.
func (gcs *GCS) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {

	// Get the object handlea and return a reader based on the object handle.
	r, err := gcs.bucket.Object(filename).NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return r, nil
}
