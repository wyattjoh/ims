package ims

import (
	"fmt"
	"image"
	"io"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/disintegration/imaging"
	negronilogrus "github.com/meatballhat/negroni-logrus"
	"github.com/pkg/errors"
	"github.com/urfave/negroni"
)

// ProcessImage uses the github.com/disintegration/imaging lib to perform the
// image transformations.
func ProcessImage(timeout time.Duration, input io.Reader, w http.ResponseWriter, r *http.Request) error {
	srcImage, format, err := image.Decode(input)
	if err != nil {
		return errors.Wrap(err, "can't decode the image")
	}

	var filter imaging.ResampleFilter
	switch r.URL.Query().Get("resize-filter") {
	case "lanczos":
		filter = imaging.Lanczos
	case "nearest":
		filter = imaging.NearestNeighbor
	case "linear":
		filter = imaging.Linear
	case "netravali":
		filter = imaging.MitchellNetravali
	case "box":
		filter = imaging.Box
	default:
		filter = imaging.Lanczos
	}

	width, err := strconv.Atoi(r.URL.Query().Get("width"))
	if err == nil {
		srcImage = imaging.Resize(srcImage, width, 0, filter)
	} else {
		height, err := strconv.Atoi(r.URL.Query().Get("height"))
		if err == nil {
			srcImage = imaging.Resize(srcImage, 0, height, filter)
		}
	}

	if timeout != 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int64(timeout.Seconds())))
		w.Header().Set("Expires", time.Now().Add(timeout).Format(http.TimeFormat))
	}

	w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))

	encoder := GetEncoder(format, r)
	if err := encoder.Encode(srcImage, w); err != nil {
		return errors.Wrap(err, "can't encode the image")
	}

	return nil
}

// GetFilename fetches the filename from the request path.
func GetFilename(r *http.Request) (string, error) {

	// We expect that the router sends us requests in the form `/:filename`
	// so we check to see if the path contains the image url that we want to
	// parse. In this case, we check to see that the path is at least 9 characters
	// long, which will ensure that the filename has at least 1 character.
	if len(r.URL.Path) < 2 {
		return "", errors.New("filename too short")
	}

	return r.URL.Path[1:], nil
}

// HandleFileSystemResize performs the actual resizing by loading the image
// from the filesystem.
func HandleFileSystemResize(timeout time.Duration, dir http.Dir) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Extract the filename from the request.
		filename, err := GetFilename(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Try to open the image from the virtual filesystem.
		f, err := dir.Open(filename)
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		// If an error occurred during the image processing, return with an internal
		// server error.
		if err := ProcessImage(timeout, f, w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// HandleOriginResize performs the actual resizing by loading the image
// from the origin.
func HandleOriginResize(timeout time.Duration, originURL *url.URL) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Extract the filename from the request.
		filename, err := GetFilename(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Parse the incomming url.
		filenameURL, err := url.Parse(filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Resolve it relative to the origin url.
		fileURL := originURL.ResolveReference(filenameURL)

		// TODO: improve, this is quite naive.

		// Perform the GET to the origin server.
		req, err := http.NewRequest("GET", fileURL.String(), nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer res.Body.Close()

		// If an error occurred during the image processing, return with an internal
		// server error.
		if err := ProcessImage(timeout, res.Body, w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// Serve creates and starts a new server to provide image resizing services.
func Serve(addr string, debug bool, directory, origin string, timeout time.Duration) error {
	mux := http.NewServeMux()

	if timeout != 0 {
		logrus.WithField("timeout", timeout.String()).Debug("cache headers enabled")
	} else {
		logrus.Debug("cache headers disabled")
	}

	// When debug mode is enabled, mount the debug handlers on this router.
	if debug {
		mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	}

	// By default, we'll try to use the directory resize, otherwise, if the origin
	// url is provided, use it.
	if origin == "" {
		logrus.WithField("directory", directory).Debug("serving from the filesystem")
		mux.HandleFunc("/", HandleFileSystemResize(timeout, http.Dir(directory)))
	} else {
		logrus.WithField("origin", origin).Debug("serving from the origin")

		originURL, err := url.Parse(origin)
		if err != nil {
			return errors.Wrap(err, "can't parse the origin url")
		}

		mux.HandleFunc("/", HandleOriginResize(timeout, originURL))
	}

	// Create the negroni middleware bundle.
	n := negroni.New(negroni.NewRecovery(), negronilogrus.NewMiddleware())

	// Attach the mux.
	n.UseHandler(mux)

	logrus.Debugf("Now listening on %s", addr)
	return http.ListenAndServe(addr, n)
}
