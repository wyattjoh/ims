# ims

[![Build Status](https://travis-ci.org/wyattjoh/ims.svg?branch=master)](https://travis-ci.org/wyattjoh/ims)
[![Go Doc](https://godoc.org/github.com/wyattjoh/ims/lib?status.svg)](http://godoc.org/github.com/wyattjoh/ims/lib)
[![Go Report](https://goreportcard.com/badge/github.com/wyattjoh/ims)](https://goreportcard.com/report/github.com/wyattjoh/ims)

The [ims](https://github.com/wyattjoh/ims) (image manipulation service) is
designed to assist with performing image transformations and optimizations on
the fly using a full Go solution provided by:
[github.com/disintegration/imaging](https://github.com/disintegration/imaging).

The application is also fitted with pprof for performance profiling, refer to
[golang.org/pkg/net/http/pprof](https://golang.org/pkg/net/http/pprof/) for
usage information.

## Usage

You can use the standard Go utility to get the binary and compile it yourself:

```bash
go get github.com/wyattjoh/ims/...
```

Or from Homebrew:

```bash
brew install wyattjoh/stable/ims
```

Running is as easy as running:

```bash
ims
```

Which will serve images from the current working directory at
`http://127.0.0.1:8080/image.jpg`. _(Note: `image.jpg` just refers to an image
url, and does not now exist, to perform a test, grab any image, throw it in the
current working directory and try it out with that url)_.

You can also use the [wyattjoh/ims](https://hub.docker.com/r/wyattjoh/ims/)
Docker image.

The default behavior is to serve images out of the present working directory,
but it can also be changed to another folder or to an origin server for it to
make the request to.

This application provides no transformation caching support, but will attach
cache-friendly headers, it is recommend that when deploying in production you
do so behind a service like [Varnish](https://www.varnish-cache.org/) or a CDN
like [Fastly](https://www.fastly.com/).

The ims application can be used as such:

```
NAME:
   ims - Image Manipulation Server

USAGE:
   ims [global options] command [command options] [arguments...]
   
COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --listen-addr value     the address to listen for new connections on (default: "127.0.0.1:8080")
   --backend value         comma separated <host>,<origin> where <origin> is a pathname or a url (with scheme) to load images from or just <origin> and the host will be the listen address
   --origin-cache value    cache the origin resources based on their cache headers (:memory: for memory based cache, directory name for file based, not specified for disabled)
   --signing-secret value  when provided, will be used to verify signed image requests made to the domain
   --signing-with-path     when provided, the path will be included in the value to compute the signature
   --disable-metrics       disable the prometheus metrics
   --timeout value         used to set the cache control max age headers, set to 0 to disable (default: 15m0s)
   --cors-domain value     use to enable CORS for the specified domain (note, this is not required to use as an image service)
   --debug                 enable debug logging and pprof routes
   --json                  print logs out in JSON
   --help, -h              show help
   --version, -v           print the version
```

## Backends

Different backends are supported by [ims](https://github.com/wyattjoh/ims).

### Google Cloud Storage

If the `--backend` is specified with an origin with a `gs://` scheme,
[ims](https://github.com/wyattjoh/ims) will use the Google Cloud Storage
provider. _Note that for authentication purposes, the environment variable
`GOOGLE_APPLICATION_CREDENTIALS` must be present, refer to
[Google Application Default Credentials](https://developers.google.com/identity/protocols/application-default-credentials)
for more information._

### Minio/Amazon S3

If the `--backend` is specified with an origin with a `s3://` scheme,
[ims](https://github.com/wyattjoh/ims) will use the S3 provider. For
configuration, you must specify the following environment variables:

- `S3_ENDPOINT`: the endpoint to use as the base for the s3 client, can be a Amazon S3 endpoint or
	a [Minio](https://www.minio.io/) one.
- `S3_ACCESS_KEY_ID`: access key id.
- `S3_ACCESS_KEY_SECRET`: access key secret.
- `S3_DONT_USE_SSL`: `TRUE` if your endpoint should be accessed by http instead of https (Default: `FALSE`).

### Other HTTP/HTTPS

If the `--backend` is specified with an origin with a `http://` or `https://`
scheme, then [ims](https://github.com/wyattjoh/ims) will use the standard
http(s) based provider. This will simply perform a GET request (merging the
relative path) against the provided origin url.

### Local/Filesystem

If the `--backend` is specified with an origin without a scheme, it will be
inferred that the origin is a local folder instead. This folder may be relative,
absolute, and will not be expanded. It is therefore not recommended to use a
tilda in your paths.

## Signing

When `--signing-secret` is provided, all requests must include a `sig` query
parameter that contains the HS256 signature of the sorted query parameters
encoded as a hex string via the provided secret. This can be used to prevent
abuse of the ims, and is strongly recommended in production.

An example of signing a request in `NodeJS`:

```javascript
const Crypto = require('crypto');
const querystring = require('querystring');

const options = {
  width: 100,
  height: 200
};

const secret = '<the-secret-we-gave-ims>';

// Create the sorted query object.
let value = Object.keys(options).sort().reduce((result, key) => {
    result.push(querystring.stringify({[key]: options[key]}))
    return result;
}, []).join('&');

// If you've enabled --signing-with-path, you need to include the path component
// in your value:
//
// value = "/my-image.jpg?" + value;
//

const sig = Crypto.createHmac('sha256', secret).update(value).digest('hex');

console.log(query + '&sig=' + sig);
```

## API

Image manipulations can be applied by appending a query string with the following parameters and as
such matches the [Fastly API](https://docs.fastly.com/api/imageopto) as much as possible. These are also in the same order that they are processed.

- `crop`: crops the image in the form: `{width},{height}`
- `resize-filter`: select the resize filter to be used. Implementation is sourced via the [github.com/disintegration/imaging](https://github.com/disintegration/imaging) package and we provide the following filters:
  - `box`: Box filter (averaging pixels).
  - `netravali`: Mitchell-Netravali cubic filter (BC-spline; B=1/3; C=1/3).
  - `linear`: Linear filter.
  - `nearest`: Nearest-neighbor filter, no anti-aliasing.
  - `gaussian`: Gaussian is a Gaussian blurring Filter.
  - `lanczos` (**default**): Lanczos filter (3 lobes).
- `format`: enables source transcoding:
  - `jpeg`: converts all images to `image/jpeg` encoding with lossless compression, some additional parameters are supported:
    - `quality`: the quality out of 100 for the output image (Default: 75).
      - `png`: converts image to `image/png` encoding
      - `gif`: converts image to `image/gif` encoding
- `width`: output image width (default is the original width).
- `height`: output image height. If both `width` and `height` are provided, the
  `width` will be used instead.
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
- `sig`: Used to specify the signing signature, see [Signing](#signing) above.

## License

MIT
