package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	// Import pprof so we can measure runtime performance.
	_ "net/http/pprof"

	"github.com/disintegration/imaging"
	"gopkg.in/gographics/imagick.v3/imagick"
)

const (
	// timeout is the cache timeout used to add to requests to prevent the
	// browser from re-requesting the image.
	timeout = 15 * time.Minute

	// defaultQuality is the quality of the image used when the quality param is
	// not provided.
	defaultQuality = 80
	keyImaging     = "imaging"
	keyImagick     = "imagick"
)

// imagingMethod uses the github.com/disintegration/imaging lib to perform the
// image transformations.
func imagingMethod(f *os.File, w http.ResponseWriter, r *http.Request) error {
	srcImage, _, err := image.Decode(f)
	if err != nil {
		log.Printf("Can't decode image: %s", err)
		return err
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
		return err
	}

	return nil
}

// imagickMethod uses the gopkg.in/gographics/imagick.v3/imagick lib to perform
// image transformations.
func imagickMethod(f *os.File, w http.ResponseWriter, r *http.Request) error {
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	// Read the image file from the operating system.
	if err := mw.ReadImageFile(f); err != nil {
		log.Printf("Can't read the image: %s", err)
		return err
	}

	// Strip the image metadata, this usually contains information that is not
	// needed when displaying on the web.
	if err := mw.StripImage(); err != nil {
		log.Printf("Can't strip image metadata: %s", err)
		return err
	}

	// If a width is defined, then it will be parsed.
	qW := r.URL.Query().Get("w")
	if qW != "" {
		if newWidth, err := strconv.Atoi(qW); err == nil {
			width := mw.GetImageWidth()
			height := mw.GetImageHeight()

			// Preserve aspect ratio using the image details read from the file
			// itself.
			newHeight := uint((float32(height) * float32(newWidth)) / float32(width))

			// Perform the acutal resizing operation.
			if err := mw.ResizeImage(uint(newWidth), newHeight, imagick.FILTER_BOX); err != nil {
				log.Printf("Can't resize the image: %s", err)
				return err
			}
		}
	}

	quality, err := strconv.Atoi(r.URL.Query().Get("q"))
	if err != nil {
		quality = defaultQuality
	}

	if err := mw.SetImageCompressionQuality(uint(quality)); err != nil {
		log.Printf("Can't set image quality: %s", err)
		return err
	}

	writeCacheHeaders(w)

	// Create a blob reader.
	blobReader := bytes.NewReader(mw.GetImageBlob())

	if _, err := io.Copy(w, blobReader); err != nil {
		log.Printf("Can't write image: %s", err)
		return err
	}

	return nil
}

// Write out the cache headers for the image type we're expecting to serve,
// which for this service, will always be jpeg.
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

	// We expect that the router sends us requests in the form `/resize/:filename`
	// so we check to see if the path contains the image url that we want to
	// parse. In this case, we check to see that the path is at least 9 characters
	// long, which will ensure that the filename has at least 1 character.
	if len(r.URL.Path) < 9 {
		log.Println("Filename too short")
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	filename := r.URL.Path[8:]

	// Try to open the image from the virtual filesystem.
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

	// We expect that the file provided is an *os.File
	osf, ok := f.(*os.File)
	if !ok {
		log.Println("File wasn't a *os.File")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// We'll store the name of the image processing option used in order to
	// log it.
	var method string

	// Switch on the `m` to select the specific mode of processing to use.
	switch r.URL.Query().Get("m") {
	case keyImagick:
		method = keyImagick
		err = imagickMethod(osf, w, r)
	case keyImaging:
		method = keyImaging
		err = imagingMethod(osf, w, r)
	default:
		method = keyImagick
		err = imagickMethod(osf, w, r)
	}

	// If an error occured during the image processing, return with an internal
	// server error.
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Served %s in %s with method %s", filename, time.Now().Sub(start).String(), method)
}

func main() {
	var listenAddr = flag.String("listen-addr", "0.0.0.0:8080", "the address to listen for new connections on")

	flag.Parse()

	// Setup the imagick lib for use.
	imagick.Initialize()
	defer imagick.Terminate()

	// Serve the index.html file.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Serve everything out of static directly.
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Mount the actual image resizing handler.
	http.Handle("/resize/", imageResizingHandler{
		imagesDir: http.Dir("images"),
	})

	log.Printf("Now serving on %s\n", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatalf("Could not serve: %s", err)
	}
}
