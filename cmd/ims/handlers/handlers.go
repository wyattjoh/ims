// Package handlers provides the routes used by the `ims` binary.
package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/wyattjoh/ims/internal/image"
	"github.com/wyattjoh/ims/internal/image/provider"
	"github.com/wyattjoh/ims/internal/platform/providers"
)

// ErrFilenameTooShort is returned when the filename referenced by the request
// is too short to be used with a provider.
var ErrFilenameTooShort = errors.New("filename too short")

// getFilename fetches the filename from the request path and validates that the
// path is valid.
func getFilename(p provider.Provider, r *http.Request) (string, error) {
	// We expect that if we're dealing with a proxy provider, that we have a
	// `?url=` on the URL.
	if _, ok := p.(*provider.Proxy); ok {
		// Get the URL parameter from the filename.
		url := r.URL.Query().Get("url")

		// To include the scheme, it must be at least 7 characters:
		//
		// http://
		//
		if len(url) <= 7 {
			return "", ErrFilenameTooShort
		}

		return url, nil
	}

	// We expect that the router sends us requests in the form `/:filename`
	// so we check to see if the path contains the image url that we want to
	// parse. In this case, we check to see that the path is at least 9 characters
	// long, which will ensure that the filename has at least 1 character.
	if len(r.URL.Path) < 2 {
		return "", ErrFilenameTooShort
	}

	return r.URL.Path[1:], nil
}

// Image is the handler which loads the filename from the request, loads the
// file via the provider, and processes the image to re-encode it with caching
// headers.
func Image(timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Extract the provider from the context.
		p, ok := ctx.Value(providers.ContextKey).(provider.Provider)
		if !ok {
			logrus.Error("expected request to contain context with provider, none found")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

			return
		}

		// Extract the filename from the request.
		filename, err := getFilename(p, r)
		if err != nil {
			logrus.WithError(err).Error("could not process the filename")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

			return
		}

		// Try to get the image from the provider.
		span, ctx := opentracing.StartSpanFromContext(r.Context(), "provider.Provide")

		m, err := p.Provide(ctx, filename)
		if err != nil {
			// We got an error! Find out which one.
			if errors.Is(err, provider.ErrBadGateway) {
				http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			} else if errors.Is(err, provider.ErrFilename) {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			} else if errors.Is(err, provider.ErrNotFound) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			} else {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			logrus.WithError(err).Error("could not load the image from the provider")
			span.Finish()

			return
		}
		defer m.Close()

		span.Finish()

		// If an error occurred during the image processing, return with an internal
		// server error.
		span, ctx = opentracing.StartSpanFromContext(r.Context(), "image.Process")
		defer span.Finish()

		if err := image.Process(ctx, timeout, m, w, r.WithContext(ctx)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logrus.WithError(err).Error("could not process the image")

			return
		}
	}
}
