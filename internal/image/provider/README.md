

# provider
`import "github.com/wyattjoh/ims/internal/image/provider"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>



## <a name="pkg-index">Index</a>
* [Variables](#pkg-variables)
* [func NewGCSTransport(ctx context.Context) (http.RoundTripper, error)](#NewGCSTransport)
* [type Filesystem](#Filesystem)
  * [func (fp *Filesystem) Provide(ctx context.Context, filename string) (io.ReadCloser, error)](#Filesystem.Provide)
* [type GCS](#GCS)
  * [func NewGCS(ctx context.Context, bucket string, transport http.RoundTripper) (*GCS, error)](#NewGCS)
  * [func (gcs *GCS) Provide(ctx context.Context, filename string) (io.ReadCloser, error)](#GCS.Provide)
* [type Origin](#Origin)
  * [func NewOrigin(baseURL *url.URL, transport http.RoundTripper) *Origin](#NewOrigin)
  * [func (op *Origin) Provide(ctx context.Context, filename string) (io.ReadCloser, error)](#Origin.Provide)
* [type Provider](#Provider)
* [type S3](#S3)
  * [func NewS3(bucket string, transport http.RoundTripper) (*S3, error)](#NewS3)
  * [func (s *S3) Provide(ctx context.Context, filename string) (io.ReadCloser, error)](#S3.Provide)


#### <a name="pkg-files">Package files</a>
[filesystem.go](/src/github.com/wyattjoh/ims/internal/image/provider/filesystem.go) [gcs.go](/src/github.com/wyattjoh/ims/internal/image/provider/gcs.go) [origin.go](/src/github.com/wyattjoh/ims/internal/image/provider/origin.go) [provider.go](/src/github.com/wyattjoh/ims/internal/image/provider/provider.go) [s3.go](/src/github.com/wyattjoh/ims/internal/image/provider/s3.go) 



## <a name="pkg-variables">Variables</a>
``` go
var (
    // ErrNotFound is returned when the file could not be found on the provider.
    ErrNotFound = errors.New("not found")

    // ErrFilename is returned when the filename could not be parsed by the
    // provider.
    ErrFilename = errors.New("bad filename")

    // ErrBadGateway is returned when the upstream provider could not service the
    // request.
    ErrBadGateway = errors.New("bad gateway")
)
```


## <a name="NewGCSTransport">func</a> [NewGCSTransport](/src/target/gcs.go?s=255:323#L7)
``` go
func NewGCSTransport(ctx context.Context) (http.RoundTripper, error)
```
NewGCSTransport returns the transport used by GCS.




## <a name="Filesystem">type</a> [Filesystem](/src/target/filesystem.go?s=129:169#L1)
``` go
type Filesystem struct {
    Dir http.Dir
}
```
Filesystem provides a way to load files from the filesystem.










### <a name="Filesystem.Provide">func</a> (\*Filesystem) [Provide](/src/target/filesystem.go?s=235:325#L6)
``` go
func (fp *Filesystem) Provide(ctx context.Context, filename string) (io.ReadCloser, error)
```
Provide provides a file via the virtual http.Dir filesystem.




## <a name="GCS">type</a> [GCS](/src/target/gcs.go?s=1177:1226#L40)
``` go
type GCS struct {
    // contains filtered or unexported fields
}
```
GCS provides a way to access files from Google Cloud Storage. Credentials
used are loaded from the `GOOGLE_APPLICATION_CREDENTIALS` environment
variable.







### <a name="NewGCS">func</a> [NewGCS](/src/target/gcs.go?s=523:613#L17)
``` go
func NewGCS(ctx context.Context, bucket string, transport http.RoundTripper) (*GCS, error)
```
NewGCS will create the GCS Provider.





### <a name="GCS.Provide">func</a> (\*GCS) [Provide](/src/target/gcs.go?s=1396:1480#L47)
``` go
func (gcs *GCS) Provide(ctx context.Context, filename string) (io.ReadCloser, error)
```
Provide provides a file by making a request to Google Cloud Storage with the
specified key and then returning the response body when the request was
complete.




## <a name="Origin">type</a> [Origin](/src/target/origin.go?s=396:458#L12)
``` go
type Origin struct {
    // contains filtered or unexported fields
}
```
Origin provides a way to access files from a url.







### <a name="NewOrigin">func</a> [NewOrigin](/src/target/origin.go?s=174:243#L2)
``` go
func NewOrigin(baseURL *url.URL, transport http.RoundTripper) *Origin
```
NewOrigin returns a new Origin Provider that will return files relative to
the provided base url.





### <a name="Origin.Provide">func</a> (\*Origin) [Provide](/src/target/origin.go?s=630:716#L20)
``` go
func (op *Origin) Provide(ctx context.Context, filename string) (io.ReadCloser, error)
```
Provide provides a file by making a request to the origin server with the
specified filename and then returning the response body when the request was
complete.




## <a name="Provider">type</a> [Provider](/src/target/provider.go?s=637:734#L16)
``` go
type Provider interface {
    Provide(ctx context.Context, filename string) (io.ReadCloser, error)
}
```
Provider describes a struct that provides the "Provide" method to provide an
image from a filename.










## <a name="S3">type</a> [S3](/src/target/s3.go?s=984:1039#L30)
``` go
type S3 struct {
    // contains filtered or unexported fields
}
```
S3 is a file provider that is capable of providing files from any S3
compatible service such as Minio or Amazon S3 itself.







### <a name="NewS3">func</a> [NewS3](/src/target/s3.go?s=274:341#L7)
``` go
func NewS3(bucket string, transport http.RoundTripper) (*S3, error)
```
NewS3 returns an S3 client capable of providing files from any S3 compatible
service such as Minio or Amazon S3 itself.





### <a name="S3.Provide">func</a> (\*S3) [Provide](/src/target/s3.go?s=1087:1168#L36)
``` go
func (s *S3) Provide(ctx context.Context, filename string) (io.ReadCloser, error)
```
Provide loads the file from the S3 client.








- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
