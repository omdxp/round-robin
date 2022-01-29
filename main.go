package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Attempts int = iota
	Retry
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

var serverPool = ServerPool{}

// lb load balances the incoming request
func lb(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("[%s] %d attempts\n", r.URL.Host, attempts)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.GetNextPeer()
	if peer != nil {
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
}

// GetRetryFromContext function
func GetRetryFromContext(ctx context.Context) int {
	if retry, ok := ctx.Value(Retry).(int); ok {
		return retry
	}
	return 0
}

// GetAttemptsFromContext function
func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}
	return 0
}

func main() {
	u, _ := url.Parse("http://localhost:8080")
	rp := httputil.NewSingleHostReverseProxy(u)
	// init server and add this as handler
	_ = http.HandlerFunc(rp.ServeHTTP)

	// create a serverPool
	serverPool = ServerPool{
		backends: []*Backend{
			&Backend{
				URL: &url.URL{
					Scheme: "http",
					Host:   "localhost:8080",
				},
			},
			&Backend{
				URL: &url.URL{
					Scheme: "http",
					Host:   "localhost:8081",
				},
			},
		},
	}

	var proxy = &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = "localhost:8080"
			r.Host = "localhost:8080"
		},
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[%s] %s\n", u.Host, err.Error())
		retries := GetRetryFromContext(r.Context())
		if retries < 3 {
			select {
			case <-time.After(10 * time.Microsecond):
				ctx := context.WithValue(r.Context(), "retry", retries+1)
				proxy.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// after 3 retries, mark this backend as dead
		serverPool.backends[0].SetAlive(false)

		// if the request routing for few attempts with different backends, increase the counter
		attempts := GetAttemptsFromContext(r)
		log.Printf("[%s] %d attempts\n", u.Host, attempts)
		ctx := context.WithValue(r.Context(), "attempts", attempts+1)
		lb(w, r.WithContext(ctx))
	}

}
