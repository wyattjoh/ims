

# ims
`import "github.com/wyattjoh/ims/lib"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>



## <a name="pkg-index">Index</a>
* [func GIFEncoder(i image.Image, w http.ResponseWriter) error](#GIFEncoder)
* [func GetFilename(r *http.Request) (string, error)](#GetFilename)
* [func HandleFileSystemResize(timeout time.Duration, dir http.Dir) http.HandlerFunc](#HandleFileSystemResize)
* [func HandleOriginResize(timeout time.Duration, origin string) (http.HandlerFunc, error)](#HandleOriginResize)
* [func PNGEncode(i image.Image, w http.ResponseWriter) error](#PNGEncode)
* [func ProcessImage(timeout time.Duration, input io.Reader, w http.ResponseWriter, r *http.Request) error](#ProcessImage)
* [func Serve(addr string, debug bool, directory, origin string, timeout time.Duration) error](#Serve)
* [type Encoder](#Encoder)
  * [func GetEncoder(format string, r *http.Request) Encoder](#GetEncoder)
* [type EncoderFunc](#EncoderFunc)
  * [func (f EncoderFunc) Encode(m image.Image, w http.ResponseWriter) error](#EncoderFunc.Encode)
* [type JPEGEncoder](#JPEGEncoder)
  * [func NewJPEGEncoder(r *http.Request) JPEGEncoder](#NewJPEGEncoder)
  * [func (je JPEGEncoder) Encode(i image.Image, w http.ResponseWriter) error](#JPEGEncoder.Encode)


#### <a name="pkg-files">Package files</a>
[encoder.go](/src/github.com/wyattjoh/ims/lib/encoder.go) [server.go](/src/github.com/wyattjoh/ims/lib/server.go) 





## <a name="GIFEncoder">func</a> [GIFEncoder](/src/target/encoder.go?s=1889:1948#L67)
``` go
func GIFEncoder(i image.Image, w http.ResponseWriter) error
```
GIFEncoder takes an image and writes the encoded gif image to it.



## <a name="GetFilename">func</a> [GetFilename](/src/target/server.go?s=1679:1728#L60)
``` go
func GetFilename(r *http.Request) (string, error)
```
GetFilename fetches the filename from the request path.



## <a name="HandleFileSystemResize">func</a> [HandleFileSystemResize](/src/target/server.go?s=2245:2326#L75)
``` go
func HandleFileSystemResize(timeout time.Duration, dir http.Dir) http.HandlerFunc
```
HandleFileSystemResize performs the actual resizing by loading the image
from the filesystem.



## <a name="HandleOriginResize">func</a> [HandleOriginResize](/src/target/server.go?s=3188:3275#L109)
``` go
func HandleOriginResize(timeout time.Duration, origin string) (http.HandlerFunc, error)
```
HandleOriginResize performs the actual resizing by loading the image
from the origin.



## <a name="PNGEncode">func</a> [PNGEncode](/src/target/encoder.go?s=1528:1586#L52)
``` go
func PNGEncode(i image.Image, w http.ResponseWriter) error
```
PNGEncode takes an image and writes the encoded png image to it.



## <a name="ProcessImage">func</a> [ProcessImage](/src/target/server.go?s=339:442#L12)
``` go
func ProcessImage(timeout time.Duration, input io.Reader, w http.ResponseWriter, r *http.Request) error
```
ProcessImage uses the github.com/disintegration/imaging lib to perform the
image transformations.



## <a name="Serve">func</a> [Serve](/src/target/server.go?s=4591:4681#L160)
``` go
func Serve(addr string, debug bool, directory, origin string, timeout time.Duration) error
```
Serve creates and starts a new server to provide image resizing services.




## <a name="Encoder">type</a> [Encoder](/src/target/encoder.go?s=329:407#L10)
``` go
type Encoder interface {
    Encode(m image.Image, w http.ResponseWriter) error
}
```
Encoder describes any type that can encode with the image and response
writer.







### <a name="GetEncoder">func</a> [GetEncoder](/src/target/encoder.go?s=1036:1091#L29)
``` go
func GetEncoder(format string, r *http.Request) Encoder
```
GetEncoder parses the `m` query variable and checks to see if it is equal to
"jpeg". If it is, it uses the JPEGEncoder, otherwise, it tries to see if it
can encode the image with another format, otherwise, it just encodes it as
"jpeg".





## <a name="EncoderFunc">type</a> [EncoderFunc](/src/target/encoder.go?s=603:668#L18)
``` go
type EncoderFunc func(m image.Image, w http.ResponseWriter) error
```
EncoderFunc type is an adapter to allow the use of
ordinary functions as image Encoders. If f is a function
with the appropriate signature, EncoderFunc(f) is a
Encoder that calls f.










### <a name="EncoderFunc.Encode">func</a> (EncoderFunc) [Encode](/src/target/encoder.go?s=695:766#L21)
``` go
func (f EncoderFunc) Encode(m image.Image, w http.ResponseWriter) error
```
Encode calls f(m, w).




## <a name="JPEGEncoder">type</a> [JPEGEncoder](/src/target/encoder.go?s=2617:2657#L94)
``` go
type JPEGEncoder struct {
    Quality int
}
```
JPEGEncoder allows the encoding of JPEG's to a http.ResponseWriter.







### <a name="NewJPEGEncoder">func</a> [NewJPEGEncoder](/src/target/encoder.go?s=2324:2372#L82)
``` go
func NewJPEGEncoder(r *http.Request) JPEGEncoder
```
NewJPEGEncoder creates a new JPEGEncoder based on the input request, this
parses the `q` query variable to check to see if it needs to change the
default quality format.





### <a name="JPEGEncoder.Encode">func</a> (JPEGEncoder) [Encode](/src/target/encoder.go?s=2731:2803#L99)
``` go
func (je JPEGEncoder) Encode(i image.Image, w http.ResponseWriter) error
```
Encode writes the encoded image data out to the http.ResponseWriter.








- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
