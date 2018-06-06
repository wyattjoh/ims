package image

import (
	"context"
	"fmt"
	"image"
	"io"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/wyattjoh/ims/internal/image/encoder"
	"github.com/wyattjoh/ims/internal/image/transform"
)

// Process uses the github.com/disintegration/imaging lib to perform the
// image transformations.
func Process(ctx context.Context, timeout time.Duration, input io.Reader, w http.ResponseWriter, r *http.Request) error {
	start := time.Now()
	logrus.Debug("starting processing image")

	// Decode the image from the reader.
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "internal.image.Process.Decode")
	m, format, err := image.Decode(input)
	if err != nil {
		span.Finish()
		return errors.Wrap(err, "can't decode the image")
	}
	span.Finish()

	// Apply image transformations.
	span, ctx = opentracing.StartSpanFromContext(r.Context(), "internal.image.Process.Transform")
	tm, err := transform.Image(m, r.URL.Query())
	if err != nil {
		span.Finish()
		return err
	}
	span.Finish()

	now := time.Now()

	// Write some caching headers if needed.
	if timeout != 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int64(timeout.Seconds())))
		w.Header().Set("Expires", now.Add(timeout).Format(http.TimeFormat))
	}

	w.Header().Set("Last-Modified", now.Format(http.TimeFormat))

	span, ctx = opentracing.StartSpanFromContext(r.Context(), "internal.image.Process.Encode")
	enc := encoder.Get(format, r)
	if err := enc.Encode(tm, w); err != nil {
		span.Finish()
		return errors.Wrap(err, "can't encode the image")
	}
	span.Finish()

	logrus.WithField("latency", time.Since(start).String()).Debug("completed processing image")
	return nil
}
