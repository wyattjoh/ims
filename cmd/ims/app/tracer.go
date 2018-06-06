package app

import (
	"fmt"
	"net/http"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// Tracer adds opentracing spanning to each incoming request.
func Tracer(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	// Get the global tracing instance.
	tracer := opentracing.GlobalTracer()

	// Define the carrier, which is the headers in this case.
	carrier := opentracing.HTTPHeadersCarrier(r.Header)

	// Ignoring the error here because it's likely that the request will
	// not contain carrier information for a distributed span.
	spanCTX, _ := tracer.Extract(opentracing.HTTPHeaders, carrier)

	// Start tracing this request, this will act as the root span.
	span := tracer.StartSpan(fmt.Sprintf("HTTP %s", r.Method), ext.RPCServerOption(spanCTX))
	defer span.Finish()

	// Configure some parameters to include in the span.
	ext.HTTPMethod.Set(span, r.Method)
	ext.HTTPUrl.Set(span, r.URL.String())

	// Create a context to pass down.
	ctx := opentracing.ContextWithSpan(r.Context(), span)

	// Start the next handler.
	next(w, r.WithContext(ctx))
}
