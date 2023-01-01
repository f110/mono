package probe

import (
	"net/http"
	"time"
)

type Probe struct {
	s  *http.Server
	ch chan chan error
}

func NewProbe(addr string) *Probe {
	p := &Probe{ch: make(chan chan error)}

	m := http.NewServeMux()
	m.HandleFunc("/liveness", func(w http.ResponseWriter, req *http.Request) {})
	m.HandleFunc("/readiness", func(w http.ResponseWriter, req *http.Request) {
		c := make(chan error)
		select {
		case p.ch <- c:
		case <-time.After(100 * time.Millisecond):
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		select {
		case err := <-c:
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	})

	p.s = &http.Server{
		Addr:    addr,
		Handler: m,
	}
	go p.s.ListenAndServe()

	return p
}

func (p *Probe) Readiness() chan chan error {
	return p.ch
}
