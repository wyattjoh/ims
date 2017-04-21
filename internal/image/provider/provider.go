package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
)

var (
	// ErrNotFound is returned when the file could not be found on the provider.
	ErrNotFound = errors.New("not found")

	// ErrFilename is returned when the filename could not be parsed by the
	// provider.
	ErrFilename = errors.New("bad filename")

	// ErrBadGateway is returned when the upstream provider could not service the
	// request.
	ErrBadGateway = errors.New("bad gateway")
)

//==============================================================================

// Provider describes a struct that provides the "Provide" method to provide an
// image from a filename.
type Provider interface {
	Provide(ctx context.Context, filename string) (io.ReadCloser, error)
}

//==============================================================================

// Filesystem provides a way to load files from the filesystem.
type Filesystem struct {
	Dir http.Dir
}

// Provide provides a file via the virtual http.Dir filesystem.
func (fp Filesystem) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {

	// Try to open the image from the virtual filesystem.
	f, err := fp.Dir.Open(filename)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return f, nil
}

//==============================================================================

// Origin provides a way to access files from a url.
type Origin struct {
	URL *url.URL
}

// Provide provides a file by making a request to the origin server with the
// specified filename and then returning the response body when the request was
// complete.
func (op Origin) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {

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
