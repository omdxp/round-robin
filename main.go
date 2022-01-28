package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
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

func main() {
	u, _ := url.Parse("http://localhost:8080")
	rp := httputil.NewSingleHostReverseProxy(u)
	// init server and add this as handler
	http.HandlerFunc(rp.ServeHTTP)
}
