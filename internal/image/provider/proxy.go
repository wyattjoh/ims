package provider

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// NewProxy returns a new Proxy Provider that will return files.
func NewProxy(transport http.RoundTripper) *Proxy {
	return &Proxy{
		client: &http.Client{
			Transport: transport,
		},
	}
}

// Proxy provides a way to access files from a url.
type Proxy struct {
	client *http.Client
}

// Provide provides a file by making a request to the server with the specified
// filename and then returning the response body when the request was complete.
func (pp *Proxy) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {
	// Parse the incomming url.
	fileURL, err := url.Parse(filename)
	if err != nil {
		return nil, ErrFilename
	}

	return pp.Handle(ctx, fileURL)
}

// Handle implements the reusable logic behind the Proxy Provider.
func (pp *Proxy) Handle(ctx context.Context, fileURL *url.URL) (io.ReadCloser, error) {
	// Perform the GET to the origin server. This takes the url passed in as the
	// origin and resolves a relative reference with the filename passed in. It
	// will not attach any query params sent on the original request.
	req, err := http.NewRequest("GET", fileURL.String(), nil)
	if err != nil {
		return nil, err
	}

	// Add the higher order context to the request. This way if the client closes
	// the connection before we get the whole image we can abort safely.
	req = req.WithContext(ctx)

	// Perform the request with the attached client.
	res, err := pp.client.Do(req)
	if err != nil {
		return nil, err
	}

	// If the code was explicitly a 404, return in kind.
	if res.StatusCode == 404 {
		return nil, ErrNotFound
	}

	// If the code wasn't a 200, then return that there was a bad gateway.
	if res.StatusCode != 200 {
		return nil, ErrBadGateway
	}

	return res.Body, nil
}
