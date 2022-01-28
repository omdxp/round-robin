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

// GetNextPeer returns next active perrt to take a connection
func (sp *ServerPool) GetNextPeer() *Backend {
	// loop entire backends to find out an Alive one
	next := sp.NextIndex()
	l := len(sp.backends) + next // start from next add move a full cycle
	for i := next; i < l; i++ {
		idx := i % len(sp.backends)     // start from next and move a full cycle
		if sp.backends[idx].IsAlive() { // if we have an alive backend, use it and store if its not the original one
			if idx != next {
				atomic.StoreUint64(&sp.current, uint64(idx))
			}
			return sp.backends[idx]
		}
	}
	return nil
}

// SetAlive for this backend
func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

// IsAlive returns true backend is alive
func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.Alive
	b.mux.RUnlock()
	return
}

// lb load balances the incoming request
func (sp *ServerPool) lb(w http.ResponseWriter, r *http.Request) {
	peer := sp.GetNextPeer()
	if peer != nil {
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
}

func main() {
	u, _ := url.Parse("http://localhost:8080")
	rp := httputil.NewSingleHostReverseProxy(u)
	// init server and add this as handler
	_ = http.HandlerFunc(rp.ServeHTTP)

}
