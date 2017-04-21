package image

import (
	"io"
	"net/http"
	"net/url"
	"os"
)

// Provider describes a struct that provides the "Provide" method to provide an
// image from a filename.
type Provider interface {
	Provide(filename string) (io.ReadCloser, error)
}

// FilesystemProvider provides a way to load files from the filesystem.
type FilesystemProvider struct {
	Dir http.Dir
}

// Provide provides a file via the virtual http.Dir filesystem.
func (fp FilesystemProvider) Provide(filename string) (io.ReadCloser, error) {

	// Try to open the image from the virtual filesystem.
	f, err := fp.Dir.Open(filename)
	if err != nil {
		if perr, ok := err.(*os.PathError); ok {
			return nil, perr
		}

		return nil, err
	}

	return f, nil
}

// OriginProvider provides a way to access files from a url.
type OriginProvider struct {
	URL *url.URL
}

// Provide provides a file by making a request to the origin server with the
// specified filename and then returning the response body when the request was
// complete.
func (op OriginProvider) Provide(filename string) (io.ReadCloser, error) {

	// Parse the incomming url.
	filenameURL, err := url.Parse(filename)
	if err != nil {
		return nil, err
	}

	// Resolve it relative to the origin url.
	fileURL := op.URL.ResolveReference(filenameURL)

	// TODO: improve, this is quite naive.

	// Perform the GET to the origin server.
	req, err := http.NewRequest("GET", fileURL.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}
