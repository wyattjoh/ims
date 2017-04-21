package image

import (
	"fmt"
	"image"
	"io"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

// Process uses the github.com/disintegration/imaging lib to perform the
// image transformations.
func Process(timeout time.Duration, input io.Reader, w http.ResponseWriter, r *http.Request) error {
	start := time.Now()
	logrus.Debug("starting processing image")

	// Decode the image from the reader.
	m, format, err := image.Decode(input)
	if err != nil {
		return errors.Wrap(err, "can't decode the image")
	}

	// Apply image transformations.
	tm, err := TransformImage(m, r.URL.Query())
	if err != nil {
		return err
	}

	// Write some caching headers if needed.
	if timeout != 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int64(timeout.Seconds())))
		w.Header().Set("Expires", time.Now().Add(timeout).Format(http.TimeFormat))
	}

	w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))

	encoder := GetEncoder(format, r)
	if err := encoder.Encode(tm, w); err != nil {
		return errors.Wrap(err, "can't encode the image")
	}

	logrus.WithField("latency", time.Since(start).String()).Debug("completed processing image")
	return nil
}
