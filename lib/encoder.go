package ims

import (
	"image"
	"net/http"

	"github.com/wyattjoh/ims/lib/gif"
	"github.com/wyattjoh/ims/lib/jpeg"
	"github.com/wyattjoh/ims/lib/png"
)

// GetEncoder parses the `m` query variable and checks to see if it is equal to
// "jpeg". If it is, it uses the jpeg.Encoder, otherwise, it tries to see if it
// can encode the image with another format, otherwise, it just encodes it as
// "jpeg".
func GetEncoder(format string, r *http.Request) Encoder {
	switch r.URL.Query().Get("format") {
	case "jpeg":
		return jpeg.NewEncoder(r)
	case "png":
		return EncoderFunc(png.Encode)
	case "gif":
		return EncoderFunc(gif.Encode)
	}

	switch format {
	case "jpeg":
		return jpeg.NewEncoder(r)
	case "png":
		return EncoderFunc(png.Encode)
	case "gif":
		return EncoderFunc(gif.Encode)
	default:
		return jpeg.NewEncoder(r)
	}
}

// Encoder describes any type that can encode with the image and response
// writer.
type Encoder interface {
	Encode(m image.Image, w http.ResponseWriter) error
}

// EncoderFunc type is an adapter to allow the use of
// ordinary functions as image Encoders. If f is a function
// with the appropriate signature, EncoderFunc(f) is a
// Encoder that calls f.
type EncoderFunc func(m image.Image, w http.ResponseWriter) error

// Encode calls f(m, w).
func (f EncoderFunc) Encode(m image.Image, w http.ResponseWriter) error {
	return f(m, w)
}
