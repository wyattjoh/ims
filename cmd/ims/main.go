package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gopkg.in/gographics/imagick.v3/imagick"

	"github.com/disintegration/imaging"
)

const (
	timeout        = 15 * time.Minute
	defaultQuality = 80
	keyImaging     = "imaging"
	keyImagick     = "imagick"
)

func imagingMethod(f *os.File, w http.ResponseWriter, r *http.Request) {
	srcImage, _, err := image.Decode(f)
	if err != nil {
		log.Printf("Can't decode image: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var destImg *image.NRGBA

	width, err := strconv.Atoi(r.URL.Query().Get("w"))
	if err != nil {
		destImg = imaging.Clone(srcImage)
	} else {
		destImg = imaging.Resize(srcImage, width, 0, imaging.Box)
	}

	quality, err := strconv.Atoi(r.URL.Query().Get("q"))
	if err != nil {
		quality = defaultQuality
	}

	writeCacheHeaders(w)

	if err := jpeg.Encode(w, destImg, &jpeg.Options{
		Quality: quality,
	}); err != nil {
		log.Printf("Can't encode image: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func imagickMethod(f *os.File, w http.ResponseWriter, r *http.Request) {
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	if err := mw.ReadImageFile(f); err != nil {
		log.Printf("Can't read the image: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := mw.StripImage(); err != nil {
		log.Printf("Can't strip image metadata: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newWidth, err := strconv.Atoi(r.URL.Query().Get("w"))
	if err == nil {
		width := mw.GetImageWidth()
		height := mw.GetImageHeight()

		// Preserve aspect ratio.
		newHeight := uint((float32(height) * float32(newWidth)) / float32(width))

		if err := mw.ResizeImage(uint(newWidth), newHeight, imagick.FILTER_BOX); err != nil {
			log.Printf("Can't resize the image: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	quality, err := strconv.Atoi(r.URL.Query().Get("q"))
	if err != nil {
		quality = defaultQuality
	}

	if err := mw.SetImageCompressionQuality(uint(quality)); err != nil {
		log.Printf("Can't set image quality: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeCacheHeaders(w)

	// Create a blob reader.
	blobReader := bytes.NewReader(mw.GetImageBlob())

	if _, err := io.Copy(w, blobReader); err != nil {
		log.Printf("Can't write image: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func writeCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int64(timeout.Seconds())))
	w.Header().Set("Last-Modified", time.Now().String())
	w.WriteHeader(http.StatusOK)
}

type imageResizingHandler struct {
	imagesDir http.Dir
}

func (irh imageResizingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	filename := r.URL.Path[len("/resize/"):]

	if len(filename) < 1 {
		log.Printf("Filename %s too short", filename)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	f, err := irh.imagesDir.Open(filename)
	if err != nil {
		log.Printf("Can't open file: %s", err)
		if _, ok := err.(*os.PathError); ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	osf, ok := f.(*os.File)
	if !ok {
		log.Println("File wasn't a *os.File")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var method string

	switch r.URL.Query().Get("m") {
	case keyImagick:
		method = keyImagick
		imagickMethod(osf, w, r)
	case keyImaging:
		method = keyImaging
		imagingMethod(osf, w, r)
	default:
		method = keyImagick
		imagickMethod(osf, w, r)
	}

	log.Printf("Served %s in %s with method %s", filename, time.Now().Sub(start).String(), method)
}

func main() {
	imagick.Initialize()
	defer imagick.Terminate()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	handler := imageResizingHandler{
		imagesDir: http.Dir("images"),
	}

	http.Handle("/resize/", handler)

	log.Println("Now serving on http://127.0.0.1:8080/")
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
