// Package server is the cloudboxd HTTP surface. Everything the CLI and SCM
// integration do goes through here (ADR 0004: server-side authority).
package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"cloudbox/internal/cluster"
	"cloudbox/internal/core"
	"cloudbox/internal/sim"
)

type Server struct {
	world *sim.World
	core  *core.Core
	mux   *http.ServeMux
}

func New(world *sim.World) *Server {
	s := &Server{}
	s.reset(world)
	s.routes()
	return s
}

func (s *Server) reset(world *sim.World) {
	s.world = world
	s.core = core.New(world)
}

func (s *Server) routes() {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /healthz", s.healthz)

	// /simctl/* is the sim driver's test-arrangement surface (ADR 0007):
	// conjure clusters and out-of-band state the way Given-steps need. It is
	// only wired when the sim driver is in play; a kube-driver server never
	// registers these routes.
	s.mux.HandleFunc("POST /simctl/reset", s.simReset)
	s.mux.HandleFunc("POST /simctl/clusters", s.simCreateCluster)
	s.mux.HandleFunc("POST /simctl/clusters/{cluster}/objects", s.simApplyRaw)
	s.mux.HandleFunc("GET /simctl/clusters/{cluster}/objects", s.simGetRaw)

	s.mux.HandleFunc("POST /v1/setup", s.setup)
	s.mux.HandleFunc("GET /v1/clusters/{cluster}/crds", s.listCRDs)
	s.mux.HandleFunc("POST /v1/applications", s.createApplication)
	s.mux.HandleFunc("GET /v1/applications/{app}", s.getApplication)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// --- plumbing ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var ce *core.Error
	if errors.As(err, &ce) {
		writeJSON(w, ce.Status, map[string]string{"error": ce.Message})
		return
	}
	writeJSON(w, 500, map[string]string{"error": err.Error()})
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return v, false
	}
	return v, true
}

// --- health & simctl ---

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) simReset(w http.ResponseWriter, _ *http.Request) {
	s.reset(sim.NewWorld())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) simCreateCluster(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Name      string `json:"name"`
		Enforcing *bool  `json:"enforcing"`
	}](w, r)
	if !ok {
		return
	}
	enforcing := true
	if req.Enforcing != nil {
		enforcing = *req.Enforcing
	}
	s.world.CreateCluster(req.Name, enforcing)
	writeJSON(w, 201, map[string]string{"name": req.Name})
}

func (s *Server) simApplyRaw(w http.ResponseWriter, r *http.Request) {
	cl, ok := s.world.Cluster(r.PathValue("cluster"))
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "cluster not known"})
		return
	}
	obj, decoded := decode[struct {
		Manifest map[string]any `json:"manifest"`
	}](w, r)
	if !decoded {
		return
	}
	parsed := cluster.FromManifest(obj.Manifest)
	cl.ApplyRaw(parsed)
	writeJSON(w, 201, parsed)
}

func (s *Server) simGetRaw(w http.ResponseWriter, r *http.Request) {
	cl, ok := s.world.Cluster(r.PathValue("cluster"))
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "cluster not known"})
		return
	}
	q := r.URL.Query()
	found, ok := cl.GetRaw(q.Get("namespace"), q.Get("kind"), q.Get("name"))
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "object not found"})
		return
	}
	writeJSON(w, 200, found)
}

// --- v1 ---

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Cluster string `json:"cluster"`
	}](w, r)
	if !ok {
		return
	}
	if err := s.core.Setup(req.Cluster); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "installed"})
}

func (s *Server) listCRDs(w http.ResponseWriter, r *http.Request) {
	crds, err := s.core.InstalledCRDs(r.PathValue("cluster"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"crds": crds})
}

func (s *Server) createApplication(w http.ResponseWriter, r *http.Request) {
	app, ok := decode[core.Application](w, r)
	if !ok {
		return
	}
	if err := s.core.CreateApplication(&app); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, app)
}

func (s *Server) getApplication(w http.ResponseWriter, r *http.Request) {
	app, err := s.core.GetApplication(r.PathValue("app"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, app)
}
