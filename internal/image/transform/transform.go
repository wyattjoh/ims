package transform

import (
	"image"
	"net/url"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
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
	//  // Parse the EXIF data and perform a rotation automatically.
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

// =============================================================================

// CropImage performs cropping operations based on the api described:
// https://docs.fastly.com/api/imageopto/crop
func CropImage(m image.Image, crop string) image.Image {
	// This assumes that the crop string contains the following form:
	//   {width},{height}
	// And will anchor it to the center point.
	if wh := strings.Split(crop, ","); len(wh) == 2 {
		width, err := strconv.Atoi(wh[0])
		if err != nil {
			return m
		}

		height, err := strconv.Atoi(wh[1])
		if err != nil {
			return m
		}

		return imaging.CropCenter(m, width, height)
	}

	return m
}

// =============================================================================

// GetResizeDimension will get the resize dimension.
func GetResizeDimension(resize string) int {
	if resize == "" {
		return 0
	}

	dimension, err := strconv.Atoi(resize)
	if err != nil {
		return 0
	}

	if dimension > 8192 {
		return 8192
	}

	return dimension
}

// =============================================================================

// GetResampleFilter gets the resample filter to use for resizing.
func GetResampleFilter(filter string) imaging.ResampleFilter {
	switch filter {
	case "lanczos":
		return imaging.Lanczos
	case "nearest":
		return imaging.NearestNeighbor
	case "linear":
		return imaging.Linear
	case "netravali":
		return imaging.MitchellNetravali
	case "box":
		return imaging.Box
	case "gaussian":
		return imaging.Gaussian
	default:
		return imaging.Lanczos
	}
}

// =============================================================================

// GetFit will return the fit parameter.
func GetFit(fit string) string {
	switch fit {
	case "cover":
		return "cover"
	case "bounds":
		return "bounds"
	default:
		return ""
	}
}

// =============================================================================

// ResizeImage resizes the image with the given resample filter.
func ResizeImage(m image.Image, w, h string, originalWidth, originalHeight int, fit string, filter imaging.ResampleFilter) image.Image {
	width := GetResizeDimension(w)
	height := GetResizeDimension(h)

	// If both width and height are provided, and we have a valid fit mode, then
	// perform a resize.
	if width > 0 && height > 0 {
		switch fit {
		case "bounds":
			// Calculate the scales relative to the originals.
			widthScale := float32(width) / float32(originalWidth)
			heightScale := float32(height) / float32(originalHeight)

			// Find the smallest scale.
			scale := widthScale
			if widthScale > heightScale {
				scale = heightScale
			}

			// Calculate the resized dimensions.
			width = int(float32(originalWidth) * scale)
			height = int(float32(originalHeight) * scale)

			// Resize the original dimensions to that scale.
			return imaging.Resize(m, width, height, filter)
		case "cover":
			return imaging.Resize(m, width, height, filter)
		}
	}

	// Resize the width if it was provided.
	if width > 0 {
		if width > originalWidth {
			// Don't resize if it's larger than the original!
			return m
		}

		return imaging.Resize(m, width, 0, filter)
	}

	// Resize the height if provided.
	if height > 0 {
		if height > originalHeight {
			// Don't resize if it's larger than the original!
			return m
		}

		return imaging.Resize(m, 0, height, filter)
	}

	return m
}

// =============================================================================

// Image transforms the image based on data found in the request. Following the
// available query params in the root README, this will parse the query params
// and apply image transformations.
func Image(m image.Image, v url.Values) (image.Image, error) {
	// Extract the width + height from the image bounds.
	width := m.Bounds().Max.X
	height := m.Bounds().Max.Y

	logrus.WithFields(logrus.Fields(map[string]interface{}{
		"width":  width,
		"height": height,
	})).Debug("image dimensions")

	// Crop the image if the crop parameter was provided.
	if crop := v.Get("crop"); crop != "" {
		// Crop the image.
		m = CropImage(m, crop)
	}

	// Resize the image if the width or height are provided.
	w := v.Get("width")
	h := v.Get("height")

	if w != "" || h != "" {
		// Get the resize filter to use.
		filter := GetResampleFilter(v.Get("resize-filter"))
		fit := GetFit(v.Get("fit"))

		m = ResizeImage(m, w, h, width, height, fit, filter)
	}

	// Reorient the image if the orientation parameter was provided.
	if orient := v.Get("orient"); orient != "" {
		// Rotate the image.
		m = RotateImage(m, orient)
	}

	// Blur the image if the parameter was provided.
	if blur := v.Get("blur"); blur != "" {
		sigma, err := strconv.ParseFloat(blur, 64)
		if err == nil && sigma > 0 {
			m = imaging.Blur(m, sigma)
		}
	}

	return m, nil
}
