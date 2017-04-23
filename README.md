# ims

[![Build Status](https://drone.wyattjoh.com/api/badges/wyatt/ims/status.svg)](https://drone.wyattjoh.com/wyatt/ims)
[![Go Doc](https://godoc.org/github.com/wyattjoh/ims/lib?status.svg)](http://godoc.org/github.com/wyattjoh/ims/lib)
[![Go Report](https://goreportcard.com/badge/github.com/wyattjoh/ims)](https://goreportcard.com/report/github.com/wyattjoh/ims)

The ims service is designed to assist with performing image transformations on
the fly using a full Go solution provided by [github.com/disintegration/imaging](https://github.com/disintegration/imaging).

The application is also fitted with pprof for performance profiling, refer to
[golang.org/pkg/net/http/pprof](https://golang.org/pkg/net/http/pprof/) for usage information.

## Usage

You can use the standard Go utility to get the binary and compile it yourself:

```bash
go get github.com/wyattjoh/ims/...
```

The default beheviour is to serve images out of a folder named "images", but it
can also be changed to another folder or to an origin server for it to make the
request to.

This application also provides no caching support, but will attach cache-friendly
headers, it is recommened that when deploying in production you do so behind a
service like [Varnish](https://www.varnish-cache.org/) or a CDN like [Fastly](https://www.fastly.com/).

The ims application can be used as such:

```
Usage of ims:
  -debug
    	enable debug logging and pprof routes
  -disable-metrics
    	disable the prometheus metrics
  -images-dir string
    	the location on the filesystem to load images from (default "images")
  -listen-addr string
    	the address to listen for new connections on (default "0.0.0.0:8080")
  -origin-url string
    	url for the origin server to pull images from
  -timeout duration
    	used to set the cache control max age headers, set to 0 to disable (default 15m0s)
```

You can also use the [wyattjoh/ims](https://hub.docker.com/r/wyattjoh/ims/) Docker image.

The API matches the Fastly API as much as possible: https://docs.fastly.com/api/imageopto/

## API

Image manipulations can be applied by appending a query string with the following parameters:

- `format`: enables source transcoding:
  - `jpeg`: converts all images to `image/jpeg` encoding with lossless compression, some additional parameters are supported:
    - `quality`: the quality out of 100 for the output image (Default: 75).
      - `png`: converts image to `image/png` encoding
      - `gif`: converts image to `image/gif` encoding
- `width`: output image width (default is the original width).
- `height`: output image height. If both `width` and `height` are provided, the
  `width` will be used instead.
- `resize-filter`: select the resize filter to be used. Implementation is sourced via the [github.com/disintegration/imaging](https://github.com/disintegration/imaging) package and we provide the following filters:
  - `box`: Box filter (averaging pixels).
  - `netravali`: Mitchell-Netravali cubic filter (BC-spline; B=1/3; C=1/3).
  - `linear`: Linear filter.
  - `nearest`: Nearest-neighbor filter, no anti-aliasing.
  - `gaussian`: Gaussian is a Gaussian blurring Filter.
  - `lanczos` (**default**): Lanczos filter (3 lobes).
- `orient`: changes the image orientation:
  - `r`: Orientate the image right.
  - `l`: Orientate the image left.
  - `h`: Flip the image horizontally.
  - `v`: Flip the image vertically.
  - `hv`: Horizontal and Vertical flip.
  - `vh`: Vertical and Vertical flip.
  - `2`: Flip the image horizontally.
  - `3`: Horizontal and Vertical flip.
  - `4`: Flip the image vertically.
  - `5`: Horizontal flip then orientate the image left.
  - `6`: Orientate the image right.
  - `7`: Horizontal flip then orientate the image right.
  - `8`: Orientate the image left.
- `blur`: produces a blurred version of the image using a Gaussian function,
  must be positive and indicates how much the image will be blurred, refers to
  the sigma value.

## License

MIT