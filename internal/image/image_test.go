package image_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	imsimage "github.com/wyattjoh/ims/internal/image"
)

func sourceImage(t *testing.T, format string) []byte {
	t.Helper()
	m := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			m.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 30), G: uint8(y * 60), A: 255})
		}
	}
	var buf bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, m, nil)
	case "png":
		err = png.Encode(&buf, m)
	case "gif":
		err = gif.Encode(&buf, m, nil)
	default:
		t.Fatalf("unsupported fixture format %q", format)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProcess(t *testing.T) {
	for _, source := range []string{"jpeg", "png", "gif"} {
		input := sourceImage(t, source)
		for _, output := range []string{"", "jpeg", "png", "gif", "unsupported"} {
			t.Run(source+"/"+output, func(t *testing.T) {
				r := httptest.NewRequest(http.MethodGet, "/image?width=3&height=2&fit=cover&orient=r&format="+output, nil)
				w := httptest.NewRecorder()
				if err := imsimage.Process(r.Context(), 0, bytes.NewReader(input), w, r); err != nil {
					t.Fatal(err)
				}
				wantFormat := output
				if output == "" || output == "unsupported" {
					wantFormat = source
				}
				if got := w.Header().Get("Content-Type"); got != "image/"+wantFormat {
					t.Fatalf("content type = %q, want image/%s", got, wantFormat)
				}
				decoded, format, err := image.Decode(w.Body)
				if err != nil {
					t.Fatal(err)
				}
				if format != wantFormat || decoded.Bounds().Size() != image.Pt(2, 3) {
					t.Fatalf("format=%s size=%v, want %s (2,3)", format, decoded.Bounds().Size(), wantFormat)
				}
				if w.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200", w.Code)
				}
			})
		}
	}
}

func TestProcessLosslessRoundTrip(t *testing.T) {
	input := sourceImage(t, "png")
	original, err := png.Decode(bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/image", nil)
	if err := imsimage.Process(r.Context(), 0, bytes.NewReader(input), w, r); err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != original.Bounds() {
		t.Fatalf("bounds = %v, want %v", got.Bounds(), original.Bounds())
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			if got.At(x, y) != original.At(x, y) {
				t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got.At(x, y), original.At(x, y))
			}
		}
	}
}

func TestProcessCacheHeaders(t *testing.T) {
	// Exercise the timestamps outside UTC, regardless of the machine's timezone.
	originalLocal := time.Local
	time.Local = time.FixedZone("test", 2*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	input := sourceImage(t, "png")
	for _, timeout := range []time.Duration{0, 90 * time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/image", nil)
			before := time.Now().Truncate(time.Second)
			if err := imsimage.Process(r.Context(), timeout, bytes.NewReader(input), w, r); err != nil {
				t.Fatal(err)
			}
			after := time.Now()
			modified, err := http.ParseTime(w.Header().Get("Last-Modified"))
			if err != nil || modified.Before(before) || modified.After(after) {
				t.Fatalf("Last-Modified = %q, err=%v; want between %v and %v", w.Header().Get("Last-Modified"), err, before, after)
			}
			if timeout == 0 {
				if w.Header().Get("Cache-Control") != "" || w.Header().Get("Expires") != "" {
					t.Fatalf("unexpected cache headers: %v", w.Header())
				}
				return
			}
			if got := w.Header().Get("Cache-Control"); got != "public, max-age=90" {
				t.Fatalf("Cache-Control = %q", got)
			}
			expires, err := http.ParseTime(w.Header().Get("Expires"))
			if err != nil || !expires.Equal(modified.Add(timeout)) {
				t.Fatalf("Expires = %q, err=%v; want %v", w.Header().Get("Expires"), err, modified.Add(timeout))
			}
		})
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestProcessDecodeErrors(t *testing.T) {
	sentinel := errors.New("read failed")
	for _, tt := range []struct {
		name  string
		input io.Reader
		cause error
	}{
		{"empty", strings.NewReader(""), nil},
		{"non image", strings.NewReader("not an image"), nil},
		{"truncated png", bytes.NewReader(sourceImage(t, "png")[:20]), nil},
		{"reader failure", io.MultiReader(bytes.NewReader(sourceImage(t, "png")[:8]), failingReader{err: sentinel}), sentinel},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/image", nil)
			err := imsimage.Process(context.Background(), time.Minute, tt.input, w, r)
			if err == nil || !strings.Contains(err.Error(), "can't decode the image") {
				t.Fatalf("got %v, want decode error", err)
			}
			if tt.cause != nil && !errors.Is(err, tt.cause) {
				t.Fatalf("got %v, want wrapped %v", err, tt.cause)
			}
			if w.Body.Len() != 0 || len(w.Header()) != 0 {
				t.Fatalf("decode failure wrote a response: headers=%v body=%q", w.Header(), w.Body.String())
			}
		})
	}
}

type failingWriter struct {
	http.ResponseWriter
	err error
}

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestProcessEncodeErrors(t *testing.T) {
	input := sourceImage(t, "png")
	sentinel := errors.New("write failed")
	for _, format := range []string{"jpeg", "png", "gif"} {
		t.Run(format, func(t *testing.T) {
			w := failingWriter{ResponseWriter: httptest.NewRecorder(), err: sentinel}
			r := httptest.NewRequest(http.MethodGet, "/image?format="+format, nil)
			err := imsimage.Process(r.Context(), 0, bytes.NewReader(input), w, r)
			if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "can't encode the image") {
				t.Fatalf("got %v, want wrapped encoding/write error", err)
			}
		})
	}
}
