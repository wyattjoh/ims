package encoder

import (
	"image"
	"net/http"

	"github.com/wyattjoh/ims/internal/image/encoder/gif"
	"github.com/wyattjoh/ims/internal/image/encoder/jpeg"
	"github.com/wyattjoh/ims/internal/image/encoder/png"
)

// Get parses the `format` query variable and uses it to see if the user has
// specified the output format, otherwise, it tries to see if it can
// encode the image with the source format, otherwise, it just encodes it as
// "jpeg".
func Get(format string, r *http.Request) Encoder {
	switch r.URL.Query().Get("format") {
	case "jpeg":
		return jpeg.NewEncoder(r)
	case "png":
		return WrapEncoderFunc(png.Encode)
	case "gif":
		return WrapEncoderFunc(gif.Encode)
	}

	switch format {
	case "jpeg":
		return jpeg.NewEncoder(r)
	case "png":
		return WrapEncoderFunc(png.Encode)
	case "gif":
		return WrapEncoderFunc(gif.Encode)
	default:
		return jpeg.NewEncoder(r)
	}
}

// Encoder describes any type that can encode with the image and response
// writer.
type Encoder interface {
	Encode(m image.Image, w http.ResponseWriter) error
}

//==============================================================================

// WrapEncoderFunc type is an adapter to allow the use of
// ordinary functions as image Encoders. If f is a function
// with the appropriate signature, WrapEncoderFunc(f) is a
// Encoder that calls f.
type WrapEncoderFunc func(m image.Image, w http.ResponseWriter) error

// Encode calls f(m, w).
func (f WrapEncoderFunc) Encode(m image.Image, w http.ResponseWriter) error {
	return f(m, w)
}
