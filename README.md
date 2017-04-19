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
go get github.com/wyattjoh/ims
```

You can also use the [wyattjoh/ims](https://hub.docker.com/r/wyattjoh/ims/) Docker image.

Any images served will be available under a `resize/` prefix, which will allow
you to attach different image manipulations via the query string. The following
query parameters are available:

- `m`: compression mode (`jpeg`, `default`):
	- `jpeg`: converts all images to `image/jpeg` encoding with lossless compression, some additional parameters are supported:
		- `q`: the quality out of 100 for the output image (Default: 80)
	- `default`: strips metadata
- `width`: output image width (default is the original width).
- `height`: output image height. If both `width` and `height` are provided, the
      `width` will be used instead.

The default beheviour is to serve images out of a folder named "images", but it
can also be changed to another folder or to an origin server for it to make the
request to.

The ims application can be used as such:

```
Usage of ims:
  -debug
        enable debug logging and pprof routes
  -images-dir string
        the location on the filesystem to load images from (default "images")
  -listen-addr string
        the address to listen for new connections on (default "0.0.0.0:8080")
  -origin-url string
        url for the origin server to pull images from
  -timeout duration
        used to set the cache control max age headers, set to 0 to disable (default 15m0s)
```

## License

MIT