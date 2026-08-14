// Package server is the cloudboxd HTTP surface. Everything the CLI and SCM
// integration do goes through here (ADR 0004: server-side authority).
package server

import (
	"net/http"

	"cloudbox/internal/sim"
)

type Server struct {
	world *sim.World
	mux   *http.ServeMux
}

func New(world *sim.World) *Server {
	s := &Server{world: world, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	// /simctl/* is the sim driver's test-arrangement surface (ADR 0007). It
	// exists only because the sim world was constructed; a kube-driver server
	// never registers it.
	s.mux.HandleFunc("POST /simctl/reset", s.simReset)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) simReset(w http.ResponseWriter, _ *http.Request) {
	s.world.Reset()
	w.WriteHeader(http.StatusNoContent)
}
