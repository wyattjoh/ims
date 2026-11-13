package encoder_test

import (
	"errors"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wyattjoh/ims/internal/image/encoder"
)

func TestGet(t *testing.T) {
	for _, tt := range []struct {
		name, source, requested, want string
	}{
		{"source jpeg", "jpeg", "", "jpeg"},
		{"source png", "png", "", "png"},
		{"source gif", "gif", "", "gif"},
		{"unknown source", "webp", "", "jpeg"},
		{"empty source", "", "", "jpeg"},
		{"override jpeg", "png", "jpeg", "jpeg"},
		{"override png", "gif", "png", "png"},
		{"override gif", "jpeg", "gif", "gif"},
		{"invalid override", "png", "webp", "png"},
		{"case sensitive override", "gif", "PNG", "gif"},
		{"both unknown", "webp", "invalid", "jpeg"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/?format="+tt.requested, nil)
			w := httptest.NewRecorder()
			if err := encoder.Get(tt.source, r).Encode(image.NewNRGBA(image.Rect(0, 0, 3, 2)), w); err != nil {
				t.Fatal(err)
			}
			if got := w.Header().Get("Content-Type"); got != "image/"+tt.want {
				t.Fatalf("content type = %q, want image/%s", got, tt.want)
			}
			got, format, err := image.Decode(w.Body)
			if err != nil {
				t.Fatal(err)
			}
			if format != tt.want || got.Bounds().Size() != image.Pt(3, 2) {
				t.Fatalf("decoded format=%s size=%v, want %s (3,2)", format, got.Bounds().Size(), tt.want)
			}
		})
	}
}

func TestWrapEncoderFunc(t *testing.T) {
	sentinel := errors.New("encoder failure")
	m := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	w := httptest.NewRecorder()
	called := 0
	wrapped := encoder.WrapEncoderFunc(func(got image.Image, response http.ResponseWriter) error {
		called++
		if got != m || response != w {
			t.Fatal("adapter did not forward the original arguments")
		}
		return sentinel
	})
	if err := wrapped.Encode(m, w); !errors.Is(err, sentinel) || called != 1 {
		t.Fatalf("err=%v calls=%d, want sentinel and one call", err, called)
	}
}

func TestGetJPEGQuality(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	encode := func(query string) string {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/?"+query, nil)
		if err := encoder.Get("png", r).Encode(m, w); err != nil {
			t.Fatal(err)
		}
		return w.Body.String()
	}
	low := encode("format=jpeg&quality=1")
	high := encode("format=jpeg&quality=100")
	if low == high || !strings.HasPrefix(low, "\xff\xd8") || !strings.HasPrefix(high, "\xff\xd8") {
		t.Fatal("JPEG selection must forward the requested quality to the encoder")
	}
}
