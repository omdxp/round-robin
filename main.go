package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

// ServerPool holds information about reachable backends
type ServerPool struct {
	backends []*Backend
	current  uint64
}

// NextIndex atomically increase the counter and return an index
func (sp *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&sp.current, uint64(1)) % uint64(len(sp.backends)))
}

func main() {
	u, _ := url.Parse("http://localhost:8080")
	rp := httputil.NewSingleHostReverseProxy(u)
	// init server and add this as handler
	http.HandlerFunc(rp.ServeHTTP)
}
