package ims

import (
	"image"
	"net/url"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/disintegration/imaging"
)

// RotateImage implements the rotating scheme described on:
// https://docs.fastly.com/api/imageopto/orient
func RotateImage(m image.Image, orient string) image.Image {
	switch orient {
	case "r":
		return imaging.Rotate270(m)
	case "l":
		return imaging.Rotate90(m)
	case "h":
		return imaging.FlipH(m)
	case "v":
		return imaging.FlipV(m)
	case "hv":
		return imaging.FlipV(imaging.FlipH(m))
	case "vh":
		return imaging.FlipH(imaging.FlipV(m))

	// case "1":
	// 	// Pending support from https://github.com/golang/go/issues/4341
	// 	return m

	case "2":
		return imaging.FlipH(m)
	case "3":
		return imaging.FlipV(imaging.FlipH(m))
	case "4":
		return imaging.FlipV(m)
	case "5":
		return imaging.Rotate90(imaging.FlipH(m))
	case "6":
		return imaging.Rotate270(m)
	case "7":
		return imaging.Rotate270(imaging.FlipH(m))
	case "8":
		return imaging.Rotate90(m)
	default:
		return m
	}
}

// TransformImage transforms the image based on data found in the request.
func TransformImage(m image.Image, v url.Values) (image.Image, error) {
	width := m.Bounds().Max.X
	height := m.Bounds().Max.Y

	logrus.WithFields(logrus.Fields(map[string]interface{}{
		"width":  width,
		"height": height,
	})).Debug("image dimensions")

	var filter imaging.ResampleFilter
	switch v.Get("resize-filter") {
	case "lanczos":
		filter = imaging.Lanczos
	case "nearest":
		filter = imaging.NearestNeighbor
	case "linear":
		filter = imaging.Linear
	case "netravali":
		filter = imaging.MitchellNetravali
	case "box":
		filter = imaging.Box
	default:
		filter = imaging.Lanczos
	}

	width, err := strconv.Atoi(v.Get("width"))
	if err == nil {
		m = imaging.Resize(m, width, 0, filter)
	} else {
		height, err := strconv.Atoi(v.Get("height"))
		if err == nil {
			m = imaging.Resize(m, 0, height, filter)
		}
	}

	orient := v.Get("orient")
	if orient != "" {

		// Rotate the image.
		m = RotateImage(m, orient)
	}

	return m, nil
}
