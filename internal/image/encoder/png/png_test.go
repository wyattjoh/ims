package png_test

import (
	"errors"
	"image"
	"image/color"
	stdpng "image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wyattjoh/ims/internal/image/encoder/png"
)

func TestEncode(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	m.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 40, B: 10, A: 128})
	m.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	w := httptest.NewRecorder()
	if err := png.Encode(m, w); err != nil {
		t.Fatal(err)
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %q", got)
	}
	decoded, err := stdpng.Decode(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != m.Bounds() {
		t.Fatalf("bounds = %v, want %v", decoded.Bounds(), m.Bounds())
	}
	for x := 0; x < 2; x++ {
		if got, want := color.NRGBAModel.Convert(decoded.At(x, 0)), m.NRGBAAt(x, 0); got != want {
			t.Errorf("pixel %d = %v, want %v", x, got, want)
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
	err := png.Encode(image.NewNRGBA(image.Rect(0, 0, 1, 1)), w)
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "can't encode the png") {
		t.Fatalf("got %v, want wrapped write error", err)
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %q", got)
	}
}
