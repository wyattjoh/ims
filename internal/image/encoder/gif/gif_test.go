package gif_test

import (
	"errors"
	"image"
	"image/color"
	stdgif "image/gif"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wyattjoh/ims/internal/image/encoder/gif"
)

func TestEncode(t *testing.T) {
	palette := color.Palette{color.Black, color.White, color.RGBA{R: 255, A: 255}}
	m := image.NewPaletted(image.Rect(0, 0, 3, 2), palette)
	copy(m.Pix, []uint8{0, 1, 2, 2, 1, 0})
	w := httptest.NewRecorder()
	if err := gif.Encode(m, w); err != nil {
		t.Fatal(err)
	}
	if got := w.Header().Get("Content-Type"); got != "image/gif" {
		t.Fatalf("content type = %q", got)
	}
	decoded, err := stdgif.Decode(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != m.Bounds() {
		t.Fatalf("bounds = %v, want %v", decoded.Bounds(), m.Bounds())
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if got, want := color.NRGBAModel.Convert(decoded.At(x, y)), color.NRGBAModel.Convert(m.At(x, y)); got != want {
				t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
			}
		}
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
	err := gif.Encode(image.NewNRGBA(image.Rect(0, 0, 1, 1)), w)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "can't encode the gif") {
		t.Fatalf("got %v, want wrapped write error", err)
	}
	if got := w.Header().Get("Content-Type"); got != "image/gif" {
		t.Fatalf("content type = %q", got)
	}
}
