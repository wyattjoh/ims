package app

import (
	"net/http"
	"net/http/pprof"
)

// MountDebug will mount pprof routes on the passed in mux.
func MountDebug(mux *http.ServeMux) {
	MountEndpoint(mux, "/debug/pprof/", http.HandlerFunc(pprof.Index))
	MountEndpoint(mux, "/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	MountEndpoint(mux, "/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	MountEndpoint(mux, "/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	MountEndpoint(mux, "/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
}
