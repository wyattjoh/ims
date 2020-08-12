package provider

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// NewOrigin returns a new Origin Provider that will return files relative to
// the provided base url.
func NewOrigin(baseURL *url.URL, transport http.RoundTripper) *Origin {
	return &Origin{
		baseURL: baseURL,
		proxy:   NewProxy(transport),
	}
}

// Origin provides a way to access files from a url.
type Origin struct {
	baseURL *url.URL
	proxy   *Proxy
}

// Provide provides a file by making a request to the origin server with the
// specified filename and then returning the response body when the request was
// complete.
func (op *Origin) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {
	// Parse the incomming url.
	filenameURL, err := url.Parse(filename)
	if err != nil {
		return nil, ErrFilename
	}

	// Resolve it relative to the origin url.
	fileURL := op.baseURL.ResolveReference(filenameURL)

	// Let the Proxy Provider handle it.
	return op.proxy.Handle(ctx, fileURL)
}
