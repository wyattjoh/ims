package provider

import (
	"context"
	"errors"
	"io"
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
