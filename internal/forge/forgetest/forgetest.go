// Package forgetest is a canned forge API for adapter tests.
package forgetest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Server answers each request with the body routed by "METHOD path?query", then "METHOD path",
// then "path" (any method); writes without a route get {}. Calls records "METHOD path body".
type Server struct {
	*httptest.Server
	mu    sync.Mutex
	calls []string
}

func New(t *testing.T, routes map[string]string) *Server {
	s := &Server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.calls = append(s.calls, strings.TrimSpace(r.Method+" "+r.URL.Path+" "+string(body)))
		s.mu.Unlock()
		for _, k := range []string{r.Method + " " + r.URL.RequestURI(), r.Method + " " + r.URL.Path, r.URL.Path} {
			if b, ok := routes[k]; ok {
				_, _ = io.WriteString(w, b)
				return
			}
		}
		if r.Method != "GET" {
			_, _ = io.WriteString(w, "{}")
			return
		}
		http.Error(w, "no route for "+r.URL.RequestURI(), 404)
	}))
	t.Cleanup(s.Close)
	return s
}

// Called reports whether a request starting with prefix ("POST /x/approve") was made.
func (s *Server) Called(prefix string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}
