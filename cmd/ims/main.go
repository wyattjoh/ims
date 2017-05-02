package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/wyattjoh/ims/cmd/ims/app"
)

const (
	// These flags are used as constants to refer to the different supported flags
	// by the application.
	flagJSON           = "json"
	flagListenAddr     = "listen-addr"
	flagDebug          = "debug"
	flagBackend        = "backend"
	flagOriginCache    = "origin-cache"
	flagDisableMetrics = "disable-metrics"
	flagTimeout        = "timeout"
	flagCORSDomain     = "cors-domain"

	defaultListenAddr = "0.0.0.0:8080"
	defaultTimeout    = 15 * time.Minute
)

// build is inserted at compile time by the linker in CI.
var build string

func init() {
	if build == "" {
		build = "dev"
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "ims"
	app.Usage = "Image Manipulation Server"
	app.Version = build
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  flagListenAddr,
			Value: defaultListenAddr,
			Usage: "the address to listen for new connections on",
		},
		cli.StringSliceFlag{
			Name:  flagBackend,
			Usage: "comma seperated <host>,<origin> where <origin> is a pathname or a url (with scheme) to load images from or just <origin> and the host will be the listen address",
		},
		cli.StringFlag{
			Name:  flagOriginCache,
			Usage: "cache the origin resources based on their cache headers (:memory: for memory based cache, directory name for file based, not specified for disabled)",
		},
		cli.BoolFlag{
			Name:  flagDisableMetrics,
			Usage: "disable the prometheus metrics",
		},
		cli.DurationFlag{
			Name:  flagTimeout,
			Value: defaultTimeout,
			Usage: "used to set the cache control max age headers, set to 0 to disable",
		},
		cli.StringSliceFlag{
			Name:  flagCORSDomain,
			Usage: "use to enable CORS for the specified domain (note, this is not required to use as an image service)",
		},
		cli.BoolFlag{
			Name:  flagDebug,
			Usage: "enable debug logging and pprof routes",
		},
		cli.BoolFlag{
			Name:  flagJSON,
			Usage: "print logs out in JSON",
		},
	}
	app.Action = ServeAction

	app.Run(os.Args)
}

// ServeAction starts the ims daemon.
func ServeAction(c *cli.Context) error {
	if !c.IsSet(flagDebug) {
		return cli.NewExitError(fmt.Sprintf("no origins specified, please use the --%s flag", flagBackend), 1)
	}

	// We want to enable debug logging as soon as we know that we're in debug
	// mode.
	if c.Bool(flagDebug) {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if c.Bool(flagJSON) {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	// Setup the server options.
	opts := &app.ServerOpts{
		Addr:           c.String(flagListenAddr),
		Debug:          c.Bool(flagDebug),
		DisableMetrics: c.Bool(flagDisableMetrics),
		Backends:       c.StringSlice(flagBackend),
		OriginCache:    c.String(flagOriginCache),
		CacheTimeout:   c.Duration(flagTimeout),
		CORSDomains:    c.StringSlice(flagCORSDomain),
	}

	if err := app.Serve(opts); err != nil {
		logrus.WithError(err).Error("could not serve")
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}
