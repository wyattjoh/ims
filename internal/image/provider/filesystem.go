package provider

import (
	"context"
	"io"
	"net/http"
	"os"
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
		if _, ok := err.(*os.PathError); ok {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return f, nil
}
