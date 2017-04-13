# ims

The ims service is designed to assist with performing image transformations on
the fly using either a full Go solution provided by github.com/disintegration/imaging
or a wrapped solution around the popular ImageMagick Library
gopkg.in/gographics/imagick.v3/imagick. The latter of course requires that the
imagemagick library is available.

## Usage

Any images served out of the `images/` directory will be available under a
`resize/` prefix, which will allow you to attach different image manipulations
via the query string.

## License

MIT