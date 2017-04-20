package jpeg

import (
	"image"
	"image/jpeg"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
)

// defaultQuality is the quality of the image used when the quality param is
// not provided.
const defaultQuality = 75

// NewEncoder creates a new Encoder based on the input request, this
// parses the `q` query variable to check to see if it needs to change the
// default quality format.
func NewEncoder(r *http.Request) Encoder {
	quality, err := strconv.Atoi(r.URL.Query().Get("quality"))
	if err != nil || quality == 0 {
		quality = defaultQuality
	}

	return Encoder{
		Quality: quality,
	}
}

// Encoder allows the encoding of JPEG's to a http.ResponseWriter.
type Encoder struct {
	Quality int
}

// Encode writes the encoded image data out to the http.ResponseWriter.
func (e Encoder) Encode(i image.Image, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "image/jpeg")

	if err := jpeg.Encode(w, i, &jpeg.Options{
		Quality: e.Quality,
	}); err != nil {
		return errors.Wrap(err, "can't encode the jpeg")
	}

	return nil
}
