package transform_test

import (
	"image"
	"image/color"
	"net/url"
	"reflect"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/wyattjoh/ims/internal/image/transform"
)

func numberedImage(width, height int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			m.SetNRGBA(x, y, color.NRGBA{R: uint8(y*width + x + 1), A: 255})
		}
	}
	return m
}

func pixels(m image.Image) []uint8 {
	var result []uint8
	for y := m.Bounds().Min.Y; y < m.Bounds().Max.Y; y++ {
		for x := m.Bounds().Min.X; x < m.Bounds().Max.X; x++ {
			result = append(result, color.NRGBAModel.Convert(m.At(x, y)).(color.NRGBA).R)
		}
	}
	return result
}

func TestRotateImage(t *testing.T) {
	source := numberedImage(3, 2)
	tests := []struct {
		orient string
		size   image.Point
		pixels []uint8
	}{
		{"r", image.Pt(2, 3), []uint8{4, 1, 5, 2, 6, 3}},
		{"l", image.Pt(2, 3), []uint8{3, 6, 2, 5, 1, 4}},
		{"h", image.Pt(3, 2), []uint8{3, 2, 1, 6, 5, 4}},
		{"v", image.Pt(3, 2), []uint8{4, 5, 6, 1, 2, 3}},
		{"hv", image.Pt(3, 2), []uint8{6, 5, 4, 3, 2, 1}},
		{"vh", image.Pt(3, 2), []uint8{6, 5, 4, 3, 2, 1}},
		{"2", image.Pt(3, 2), []uint8{3, 2, 1, 6, 5, 4}},
		{"3", image.Pt(3, 2), []uint8{6, 5, 4, 3, 2, 1}},
		{"4", image.Pt(3, 2), []uint8{4, 5, 6, 1, 2, 3}},
		{"5", image.Pt(2, 3), []uint8{1, 4, 2, 5, 3, 6}},
		{"6", image.Pt(2, 3), []uint8{4, 1, 5, 2, 6, 3}},
		{"7", image.Pt(2, 3), []uint8{6, 3, 5, 2, 4, 1}},
		{"8", image.Pt(2, 3), []uint8{3, 6, 2, 5, 1, 4}},
		{"", image.Pt(3, 2), []uint8{1, 2, 3, 4, 5, 6}},
		{"unknown", image.Pt(3, 2), []uint8{1, 2, 3, 4, 5, 6}},
	}
	for _, tt := range tests {
		t.Run(tt.orient, func(t *testing.T) {
			got := transform.RotateImage(source, tt.orient)
			if got.Bounds().Size() != tt.size || !reflect.DeepEqual(pixels(got), tt.pixels) {
				t.Fatalf("got size %v, pixels %v; want %v, %v", got.Bounds().Size(), pixels(got), tt.size, tt.pixels)
			}
			if want := []uint8{1, 2, 3, 4, 5, 6}; !reflect.DeepEqual(pixels(source), want) {
				t.Fatal("rotation mutated the source")
			}
		})
	}
}

func TestCropImage(t *testing.T) {
	source := numberedImage(5, 3)
	for _, crop := range []string{"", "bad", "1", "1,2,3", "bad,2", "2,bad", "2.5,2"} {
		t.Run("invalid/"+crop, func(t *testing.T) {
			if got := transform.CropImage(source, crop); got != source {
				t.Fatal("invalid crop should leave the source unchanged")
			}
		})
	}
	for _, tt := range []struct {
		crop   string
		size   image.Point
		pixels []uint8
	}{
		{"3,1", image.Pt(3, 1), []uint8{7, 8, 9}},
		{"1,1", image.Pt(1, 1), []uint8{8}},
		{"99,99", image.Pt(5, 3), pixels(source)},
		{"0,1", image.Point{}, nil},
	} {
		t.Run(tt.crop, func(t *testing.T) {
			got := transform.CropImage(source, tt.crop)
			if got.Bounds().Size() != tt.size || !reflect.DeepEqual(pixels(got), tt.pixels) {
				t.Fatalf("got size %v, pixels %v; want %v, %v", got.Bounds().Size(), pixels(got), tt.size, tt.pixels)
			}
		})
	}
}

func TestGetResizeDimension(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  int
	}{
		{"", 0},
		{"bad", 0},
		{"1.5", 0},
		{"0", 0},
		{"1", 1},
		{"8191", 8191},
		{"8192", 8192},
		{"8193", 8192},
		{"999999", 8192},
		{"999999999999999999999999999999999999", 0},
	} {
		t.Run(tt.input, func(t *testing.T) {
			if got := transform.GetResizeDimension(tt.input); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetResampleFilter(t *testing.T) {
	for _, tt := range []struct {
		name   string
		filter imaging.ResampleFilter
	}{
		{"lanczos", imaging.Lanczos},
		{"nearest", imaging.NearestNeighbor},
		{"linear", imaging.Linear},
		{"netravali", imaging.MitchellNetravali},
		{"box", imaging.Box},
		{"gaussian", imaging.Gaussian},
		{"", imaging.Lanczos},
		{"unknown", imaging.Lanczos},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := transform.GetResampleFilter(tt.name)
			if got.Support != tt.filter.Support {
				t.Fatalf("support = %v, want %v", got.Support, tt.filter.Support)
			}
			if tt.filter.Kernel == nil {
				if got.Kernel != nil {
					t.Fatal("expected nearest-neighbor filter without a kernel")
				}
				return
			}
			if got.Kernel == nil {
				t.Fatal("expected a resampling kernel")
			}
			for _, x := range []float64{0, 0.25, 0.75, 1, 1.5, 2.5, 3.5} {
				if got.Kernel(x) != tt.filter.Kernel(x) {
					t.Fatalf("kernel(%v) = %v, want %v", x, got.Kernel(x), tt.filter.Kernel(x))
				}
			}
		})
	}
}

func TestGetFit(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"cover", "cover"}, {"bounds", "bounds"}, {"", ""}, {"unknown", ""}, {"COVER", ""},
	} {
		t.Run(tt.input, func(t *testing.T) {
			if got := transform.GetFit(tt.input); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResizeImage(t *testing.T) {
	source := numberedImage(8, 4)
	for _, tt := range []struct {
		name, width, height, fit string
		size                     image.Point
		unchanged                bool
	}{
		{"no dimensions", "", "", "", image.Pt(8, 4), true},
		{"invalid dimensions", "bad", "bad", "cover", image.Pt(8, 4), true},
		{"zero dimensions", "0", "0", "", image.Pt(8, 4), true},
		{"negative dimensions", "-1", "-2", "", image.Pt(8, 4), true},
		{"width", "4", "", "", image.Pt(4, 2), false},
		{"height", "", "2", "", image.Pt(4, 2), false},
		{"width takes precedence", "4", "3", "", image.Pt(4, 2), false},
		{"unknown fit", "4", "3", "unknown", image.Pt(4, 2), false},
		{"no width upscale", "9", "", "", image.Pt(8, 4), true},
		{"no height upscale", "", "5", "", image.Pt(8, 4), true},
		{"bounds width constrained", "4", "4", "bounds", image.Pt(4, 2), false},
		{"bounds height constrained", "8", "2", "bounds", image.Pt(4, 2), false},
		{"cover", "3", "3", "cover", image.Pt(3, 3), false},
		{"explicit cover upscale", "10", "6", "cover", image.Pt(10, 6), false},
		{"fit requires both dimensions", "4", "", "bounds", image.Pt(4, 2), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := transform.ResizeImage(source, tt.width, tt.height, 8, 4, tt.fit, imaging.NearestNeighbor)
			if got.Bounds().Size() != tt.size {
				t.Fatalf("size = %v, want %v", got.Bounds().Size(), tt.size)
			}
			if tt.unchanged && got != source {
				t.Fatal("expected original image")
			}
		})
	}
}

func TestResizeImagePixels(t *testing.T) {
	source := numberedImage(4, 4)
	before := pixels(source)
	got := transform.ResizeImage(source, "2", "2", 4, 4, "cover", imaging.NearestNeighbor)
	want := []uint8{6, 8, 14, 16}
	if !reflect.DeepEqual(pixels(got), want) {
		t.Fatalf("pixels = %v, want %v", pixels(got), want)
	}
	if !reflect.DeepEqual(pixels(source), before) {
		t.Fatal("resize mutated the source")
	}
}

func TestImage(t *testing.T) {
	for _, tt := range []struct {
		name, query string
		size        image.Point
	}{
		{"noop", "", image.Pt(8, 4)},
		{"crop", "crop=4,2", image.Pt(4, 2)},
		{"width", "width=4", image.Pt(4, 2)},
		{"height", "height=2", image.Pt(4, 2)},
		{"bounds", "width=3&height=3&fit=bounds", image.Pt(3, 1)},
		{"cover", "width=3&height=3&fit=cover", image.Pt(3, 3)},
		{"rotate after resize", "width=3&height=1&fit=cover&orient=r", image.Pt(1, 3)},
		{"all operations", "crop=4,2&width=2&height=2&fit=cover&orient=l&blur=0.5&resize-filter=box", image.Pt(2, 2)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			values, err := url.ParseQuery(tt.query)
			if err != nil {
				t.Fatal(err)
			}
			got, err := transform.Image(numberedImage(8, 4), values)
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds().Size() != tt.size {
				t.Fatalf("size = %v, want %v", got.Bounds().Size(), tt.size)
			}
		})
	}
}

func TestImageResizeUsesCurrentDimensions(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source image.Image
		values url.Values
		want   image.Point
	}{
		{
			name:   "bounds preserves cropped aspect ratio",
			source: numberedImage(8, 4),
			values: url.Values{"crop": {"4,4"}, "width": {"4"}, "height": {"4"}, "fit": {"bounds"}},
			want:   image.Pt(4, 4),
		},
		{
			name:   "width does not upscale a crop",
			source: numberedImage(8, 4),
			values: url.Values{"crop": {"4,4"}, "width": {"6"}},
			want:   image.Pt(4, 4),
		},
		{
			name:   "height does not upscale a crop",
			source: numberedImage(8, 4),
			values: url.Values{"crop": {"4,2"}, "height": {"3"}},
			want:   image.Pt(4, 2),
		},
		{
			name:   "bounds accounts for nonzero image origin",
			source: image.NewNRGBA(image.Rect(10, 20, 18, 24)),
			values: url.Values{"width": {"4"}, "height": {"4"}, "fit": {"bounds"}},
			want:   image.Pt(4, 2),
		},
		{
			name:   "width limit accounts for nonzero image origin",
			source: image.NewNRGBA(image.Rect(10, 20, 18, 24)),
			values: url.Values{"width": {"10"}},
			want:   image.Pt(8, 4),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transform.Image(tt.source, tt.values)
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds().Size() != tt.want {
				t.Fatalf("size = %v, want %v", got.Bounds().Size(), tt.want)
			}
		})
	}
}

func TestImageCropsBeforeRotating(t *testing.T) {
	got, err := transform.Image(numberedImage(5, 3), url.Values{"crop": {"3,1"}, "orient": {"r"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Size() != image.Pt(1, 3) || !reflect.DeepEqual(pixels(got), []uint8{7, 8, 9}) {
		t.Fatalf("got size %v, pixels %v; want (1,3), [7 8 9]", got.Bounds().Size(), pixels(got))
	}
}

func TestImageBlur(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 5, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			source.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}
	source.SetNRGBA(2, 2, color.NRGBA{R: 255, A: 255})
	for _, blur := range []string{"", "bad", "0", "-1", "NaN"} {
		t.Run("ignored/"+blur, func(t *testing.T) {
			got, err := transform.Image(source, url.Values{"blur": {blur}})
			if err != nil || got != source {
				t.Fatalf("invalid blur changed source: image=%T, err=%v", got, err)
			}
		})
	}
	got, err := transform.Image(source, url.Values{"blur": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	center := color.NRGBAModel.Convert(got.At(2, 2)).(color.NRGBA)
	neighbor := color.NRGBAModel.Convert(got.At(1, 2)).(color.NRGBA)
	if got.Bounds() != source.Bounds() || center.R == 0 || center.R >= 255 || neighbor.R == 0 {
		t.Fatalf("blur did not spread the impulse: center=%v neighbor=%v bounds=%v", center, neighbor, got.Bounds())
	}
	if source.NRGBAAt(2, 2).R != 255 || source.NRGBAAt(1, 2).R != 0 {
		t.Fatal("blur mutated the source")
	}
}
