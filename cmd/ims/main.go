package main

import (
	"fmt"
	"io"
	"os"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	jaeger "github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
	"github.com/urfave/cli/v2"
	"github.com/wyattjoh/ims/cmd/ims/app"
)

const (
	// These flags are used as constants to refer to the different supported flags
	// by the application.
	flagJSON                   = "json"
	flagListenAddr             = "listen-addr"
	flagDebug                  = "debug"
	flagBackend                = "backend"
	flagOriginCache            = "origin-cache"
	flagDisableMetrics         = "disable-metrics"
	flagTimeout                = "timeout"
	flagCORSDomain             = "cors-domain"
	flagSigningSecret          = "signing-secret"
	flagIncludePathWhenSigning = "signing-with-path"
	flagTracingURI             = "tracing-uri"

	defaultListenAddr = "127.0.0.1:8080"
	defaultTimeout    = 15 * time.Minute
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app := cli.NewApp()
	app.Name = "ims"
	app.Usage = "Image Manipulation Server"
	app.Version = fmt.Sprintf("%v, commit %v, built at %v", version, commit, date)
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  flagListenAddr,
			Value: defaultListenAddr,
			Usage: "the address to listen for new connections on",
		},
		&cli.StringSliceFlag{
			Name:  flagBackend,
			Usage: "comma separated <host>,<origin> where <origin> is a pathname or a url (with scheme) to load images from or just <origin> and the host will be the listen address",
		},
		&cli.StringFlag{
			Name:  flagOriginCache,
			Usage: "cache the origin resources based on their cache headers (:memory: for memory based cache, directory name for file based, not specified for disabled)",
		},
		&cli.StringFlag{
			Name:  flagSigningSecret,
			Usage: "when provided, will be used to verify signed image requests made to the domain",
		},
		&cli.StringFlag{
			Name:  flagTracingURI,
			Usage: "when provided, will be used to send tracing information via opentracing",
		},
		&cli.BoolFlag{
			Name:  flagIncludePathWhenSigning,
			Usage: "when provided, the path will be included in the value to compute the signature",
		},
		&cli.BoolFlag{
			Name:  flagDisableMetrics,
			Usage: "disable the prometheus metrics",
		},
		&cli.DurationFlag{
			Name:  flagTimeout,
			Value: defaultTimeout,
			Usage: "used to set the cache control max age headers, set to 0 to disable",
		},
		&cli.StringSliceFlag{
			Name:  flagCORSDomain,
			Usage: "use to enable CORS for the specified domain (note, this is not required to use as an image service)",
		},
		&cli.BoolFlag{
			Name:  flagDebug,
			Usage: "enable debug logging and pprof routes",
		},
		&cli.BoolFlag{
			Name:  flagJSON,
			Usage: "print logs out in JSON",
		},
	}
	app.Action = ServeAction

	if err := app.Run(os.Args); err != nil {
		logrus.WithError(err).Fatal()
	}
}

// SetupTracing will setup the tracing using Jaeger.
func SetupTracing(tracingURI string) (opentracing.Tracer, io.Closer) {
	var sampler jaeger.Sampler
	var reporter jaeger.Reporter

	if tracingURI == "" {
		sampler = jaeger.NewConstSampler(false)
		reporter = jaeger.NewNullReporter()

		logrus.Info("not reporting tracing information, missing --tracing-uri option")
	} else {
		sampler = jaeger.NewRateLimitingSampler(100)
		sender := transport.NewHTTPTransport(tracingURI)
		reporter = jaeger.NewRemoteReporter(sender)

		logrus.WithField("tracingURI", tracingURI).Info("reporting tracing information")
	}

	return jaeger.NewTracer("ims", sampler, reporter)
}

// ServeAction starts the ims daemon.
func ServeAction(c *cli.Context) error {
	var backends []string
	if c.IsSet(flagBackend) {
		backends = c.StringSlice(flagBackend)
	} else {
		pwd, err := os.Getwd()
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "can't get the current working directory").Error(), 1)
		}
		backends = []string{pwd}
	}

	// We want to enable debug logging as soon as we know that we're in debug
	// mode.
	if c.Bool(flagDebug) {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if c.Bool(flagJSON) {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	// Configure the tracer.
	tracer, closer := SetupTracing(c.String(flagTracingURI))
	defer closer.Close()
	opentracing.InitGlobalTracer(tracer)

	// Setup the server options.
	opts := &app.ServerOpts{
		Addr:           c.String(flagListenAddr),
		Debug:          c.Bool(flagDebug),
		DisableMetrics: c.Bool(flagDisableMetrics),
		Backends:       backends,
		OriginCache:    c.String(flagOriginCache),
		CacheTimeout:   c.Duration(flagTimeout),
		CORSDomains:    c.StringSlice(flagCORSDomain),
		SigningSecret:  c.String(flagSigningSecret),
		IncludePath:    c.Bool(flagIncludePathWhenSigning),
	}

	if err := app.Serve(opts); err != nil {
		logrus.WithError(err).Error("could not serve")
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}
