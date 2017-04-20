package gif

import (
	"image"
	"image/gif"
	"net/http"

	"github.com/pkg/errors"
)

// Encode takes an image and writes the encoded gif image to it.
func Encode(i image.Image, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "image/gif")

	options := gif.Options{}

	if err := gif.Encode(w, i, &options); err != nil {
		return errors.Wrap(err, "can't encode the gif")
	}

	return nil
}
