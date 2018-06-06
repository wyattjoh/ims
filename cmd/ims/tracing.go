package main

import (
	"io"

	"github.com/Sirupsen/logrus"
	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
)

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
