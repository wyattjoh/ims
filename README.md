# ims

[![Go Doc](https://godoc.org/github.com/wyattjoh/ims?status.svg)](http://godoc.org/github.com/wyattjoh/ims)
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

Any images served out of the `images/` directory will be available under a
`resize/` prefix, which will allow you to attach different image manipulations
via the query string. The following query parameters are available:

- `m`: compression mode (`jpeg`, `default`):
	- `jpeg`: converts all images to `image/jpeg` encoding with lossless compression, some additional parameters are supported:
		- `q`: the quality out of 100 for the output image (Default: 80)
	- `default`: strips metadata
- `w`: output image width (default is the original width).

## License

MIT