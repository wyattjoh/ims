// Package routes provides the routes used by the `ims` binary.
package routes

import (
	"errors"
	"net/http"
	"time"

	"github.com/wyattjoh/ims/internal/image"
	"github.com/wyattjoh/ims/internal/image/provider"
)

// getFilename fetches the filename from the request path and validates that the
// path is valid.
func getFilename(r *http.Request) (string, error) {

	// We expect that the router sends us requests in the form `/:filename`
	// so we check to see if the path contains the image url that we want to
	// parse. In this case, we check to see that the path is at least 9 characters
	// long, which will ensure that the filename has at least 1 character.
	if len(r.URL.Path) < 2 {
		return "", errors.New("filename too short")
	}

	return r.URL.Path[1:], nil
}

// Resize is the handler which loads the filename from the request, loads the
// file via the provider, and processes the image to re-encode it with caching
// headers.
func Resize(timeout time.Duration, p provider.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Extract the filename from the request.
		filename, err := getFilename(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Try to get the image from the provider.
		m, err := p.Provide(filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If an error occurred during the image processing, return with an internal
		// server error.
		if err := image.Process(timeout, m, w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
