package profiling

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServesProfiles(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()
	for path, want := range map[string]int{
		"/debug/pprof/":          http.StatusOK,
		"/debug/pprof/heap":      http.StatusOK, // named profiles go through Index
		"/debug/pprof/goroutine": http.StatusOK,
		"/events/1/bookings":     http.StatusNotFound,
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s: %d, want %d", path, resp.StatusCode, want)
		}
	}
}
