package png

import (
	"image"
	"image/png"
	"net/http"

	"github.com/pkg/errors"
)

// Encode takes an image and writes the encoded png image to it.
func Encode(i image.Image, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "image/png")

	encoder := png.Encoder{
		CompressionLevel: png.BestCompression,
	}

	if err := encoder.Encode(w, i); err != nil {
		return errors.Wrap(err, "can't encode the png")
	}

	return nil
}
