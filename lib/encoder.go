package ims

import (
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
)

// defaultQuality is the quality of the image used when the quality param is
// not provided.
const defaultQuality = 75

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

// GetEncoder parses the `m` query variable and checks to see if it is equal to
// "jpeg". If it is, it uses the JPEGEncoder, otherwise, it tries to see if it
// can encode the image with another format, otherwise, it just encodes it as
// "jpeg".
func GetEncoder(format string, r *http.Request) Encoder {
	switch r.URL.Query().Get("format") {
	case "jpeg":
		return NewJPEGEncoder(r)
	case "png":
		return EncoderFunc(PNGEncode)
	case "gif":
		return EncoderFunc(GIFEncoder)
	}

	switch format {
	case "jpeg":
		return NewJPEGEncoder(r)
	case "png":
		return EncoderFunc(PNGEncode)
	case "gif":
		return EncoderFunc(GIFEncoder)
	default:
		return NewJPEGEncoder(r)
	}
}

// PNGEncode takes an image and writes the encoded png image to it.
func PNGEncode(i image.Image, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "image/png")

	encoder := png.Encoder{
		CompressionLevel: png.BestCompression,
	}

	if err := encoder.Encode(w, i); err != nil {
		return errors.Wrap(err, "can't encode the png")
	}

	return nil
}

// GIFEncoder takes an image and writes the encoded gif image to it.
func GIFEncoder(i image.Image, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "image/gif")

	options := gif.Options{}

	if err := gif.Encode(w, i, &options); err != nil {
		return errors.Wrap(err, "can't encode the gif")
	}

	return nil
}

// NewJPEGEncoder creates a new JPEGEncoder based on the input request, this
// parses the `q` query variable to check to see if it needs to change the
// default quality format.
func NewJPEGEncoder(r *http.Request) JPEGEncoder {
	quality, err := strconv.Atoi(r.URL.Query().Get("quality"))
	if err != nil || quality == 0 {
		quality = defaultQuality
	}

	return JPEGEncoder{
		Quality: quality,
	}
}

// JPEGEncoder allows the encoding of JPEG's to a http.ResponseWriter.
type JPEGEncoder struct {
	Quality int
}

// Encode writes the encoded image data out to the http.ResponseWriter.
func (je JPEGEncoder) Encode(i image.Image, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "image/jpeg")

	if err := jpeg.Encode(w, i, &jpeg.Options{
		Quality: je.Quality,
	}); err != nil {
		return errors.Wrap(err, "can't encode the jpeg")
	}

	return nil
}
