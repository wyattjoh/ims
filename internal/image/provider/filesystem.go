package provider

import (
	"context"
	"io"
	"net/http"
	"os"

	"github.com/pkg/errors"
)

// Filesystem provides a way to load files from the filesystem.
type Filesystem struct {
	Dir http.Dir
}

// Provide provides a file via the virtual http.Dir filesystem.
func (fp *Filesystem) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {
	// Try to open the image from the virtual filesystem.
	f, err := fp.Dir.Open(filename)
	if err != nil {
		if errors.As(err, &os.PathError{}) {
			return nil, ErrNotFound
		}

		return nil, errors.Wrap(err, "cannot get file from filesystem")
	}

	return f, nil
}
