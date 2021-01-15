package provider

import (
	"context"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

// NewGCSTransport returns the transport used by GCS.
func NewGCSTransport(ctx context.Context) (http.RoundTripper, error) {
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadOnly)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create token source")
	}

	return oauth2.NewClient(ctx, ts).Transport, nil
}

// NewGCS will create the GCS Provider.
func NewGCS(ctx context.Context, bucket string, transport http.RoundTripper) (*GCS, error) {
	// Create the options for the client.
	opts := []cloud.ClientOption{
		cloud.WithBaseHTTP(&http.Client{
			Transport: transport,
		}),
	}

	// Creates a storage client for the requests made by this server.
	client, err := storage.NewClient(ctx, opts...)
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
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, ErrNotFound
		}

		return nil, errors.Wrap(err, "cannot get file from provider")
	}

	return r, nil
}
