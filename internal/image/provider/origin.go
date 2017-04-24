package provider

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// Origin provides a way to access files from a url.
type Origin struct {
	URL *url.URL
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
	fileURL := op.URL.ResolveReference(filenameURL)

	// TODO: improve, this is quite naive.

	// Perform the GET to the origin server.
	req, err := http.NewRequest("GET", fileURL.String(), nil)
	if err != nil {
		return nil, err
	}

	// Add the higher order context to the request. This way if the client closes
	// the connection before we get the whole image we can abort safely.
	req = req.WithContext(ctx)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == 404 {
		return nil, ErrNotFound
	}

	if res.StatusCode != 200 {
		return nil, ErrBadGateway
	}

	return res.Body, nil
}
