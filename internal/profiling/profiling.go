// Package profiling serves the Go runtime's profiles (net/http/pprof) for
// the lab's profiler to collect. It's its own HTTP handler, for its own
// port: never the public API's.
package profiling

import (
	"net/http"
	"net/http/pprof"
)

// Handler serves the profiles under /debug/pprof/.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
