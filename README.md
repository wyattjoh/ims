# ims

The ims service is designed to assist with performing image transformations on
the fly using either a full Go solution provided by github.com/disintegration/imaging
or a wrapped solution around the popular ImageMagick Library
gopkg.in/gographics/imagick.v3/imagick. The latter of course requires that the
imagemagick library is available.

The application is also fitted with pprof for performance profiling, refer to
https://golang.org/pkg/net/http/pprof/ for usage information.

## Usage

Any images served out of the `images/` directory will be available under a
`resize/` prefix, which will allow you to attach different image manipulations
via the query string. The following query parameters are available:

- `m`: Used to specify the type of image processing method to use, either
  (`imaging` or `imagick`).
- `q`: Used to specify the output JPEG quality (default `80`).
- `w`: Used to specify the output image width (default is the original width).

## License

MIT