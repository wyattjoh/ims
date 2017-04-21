// Package routes provides the routes used by the `ims` binary.
package routes

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/wyattjoh/ims/internal/image"
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

// FileSystemResize performs the actual resizing by loading the image
// from the filesystem.
func FileSystemResize(timeout time.Duration, dir http.Dir) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Extract the filename from the request.
		filename, err := getFilename(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Try to open the image from the virtual filesystem.
		f, err := dir.Open(filename)
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		// If an error occurred during the image processing, return with an internal
		// server error.
		if err := image.Process(timeout, f, w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// OriginResize performs the actual resizing by loading the image
// from the origin.
func OriginResize(timeout time.Duration, originURL *url.URL) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Extract the filename from the request.
		filename, err := getFilename(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Parse the incomming url.
		filenameURL, err := url.Parse(filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Resolve it relative to the origin url.
		fileURL := originURL.ResolveReference(filenameURL)

		// TODO: improve, this is quite naive.

		// Perform the GET to the origin server.
		req, err := http.NewRequest("GET", fileURL.String(), nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer res.Body.Close()

		// If an error occurred during the image processing, return with an internal
		// server error.
		if err := image.Process(timeout, res.Body, w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
