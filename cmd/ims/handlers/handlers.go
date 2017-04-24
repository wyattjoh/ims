// Package handlers provides the routes used by the `ims` binary.
package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
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

// Image is the handler which loads the filename from the request, loads the
// file via the provider, and processes the image to re-encode it with caching
// headers.
func Image(timeout time.Duration, p provider.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Extract the filename from the request.
		filename, err := getFilename(r)
		if err != nil {
			logrus.WithError(err).Error("could not process the filename")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		// Try to get the image from the provider.
		m, err := p.Provide(ctx, filename)
		if err != nil {
			switch err {
			case provider.ErrBadGateway:
				http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			case provider.ErrFilename:
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			case provider.ErrNotFound:
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			default:
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			logrus.WithError(err).Error("could not load the image from the provider")
			return
		}

		// If an error occurred during the image processing, return with an internal
		// server error.
		if err := image.Process(ctx, timeout, m, w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logrus.WithError(err).Error("could not process the image")
			return
		}
	}
}
