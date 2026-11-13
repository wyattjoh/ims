package jpeg_test

import (
	"errors"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wyattjoh/ims/internal/image/encoder/jpeg"
)

func TestNewEncoder(t *testing.T) {
	for _, tt := range []struct {
		quality string
		want    int
	}{
		{"", 75},
		{"invalid", 75},
		{"0", 75},
		{"1.5", 75},
		{"999999999999999999999999999999", 75},
		{"1", 1},
		{"50", 50},
		{"100", 100},
	} {
		t.Run(tt.quality, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/?quality="+tt.quality, nil)
			if got := jpeg.NewEncoder(r).Quality; got != tt.want {
				t.Fatalf("quality = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			m.SetNRGBA(x, y, color.NRGBA{R: 120, G: 120, B: 120, A: 255})
		}
	}
	w := httptest.NewRecorder()
	if err := (jpeg.Encoder{Quality: 100}).Encode(m, w); err != nil {
		t.Fatal(err)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content type = %q", got)
	}
	decoded, err := stdjpeg.Decode(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != m.Bounds() {
		t.Fatalf("bounds = %v, want %v", decoded.Bounds(), m.Bounds())
	}
	pixel := color.GrayModel.Convert(decoded.At(0, 0)).(color.Gray).Y
	if pixel < 118 || pixel > 122 {
		t.Fatalf("decoded intensity = %d, want approximately 120", pixel)
	}
}

type failingWriter struct {
	http.ResponseWriter
	err error
}

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestEncodeWriteError(t *testing.T) {
	sentinel := errors.New("write failed")
	w := failingWriter{ResponseWriter: httptest.NewRecorder(), err: sentinel}
	err := (jpeg.Encoder{Quality: 75}).Encode(image.NewNRGBA(image.Rect(0, 0, 1, 1)), w)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "can't encode the jpeg") {
		t.Fatalf("got %v, want wrapped write error", err)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content type = %q", got)
	}
}
