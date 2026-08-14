// Package server is the cloudboxd HTTP surface. Everything the CLI and SCM
// integration do goes through here (ADR 0004: server-side authority).
package server

import (
	"encoding/json"
	"errors"
	"fmt"
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
	s.mux.HandleFunc("PUT /v1/applications/{app}/contract", s.updateContract)
	s.mux.HandleFunc("PUT /v1/applications/{app}/secret-values", s.setSecretValue)
	s.mux.HandleFunc("POST /v1/sandboxes", s.createSandbox)
	s.mux.HandleFunc("GET /v1/sandboxes/{sandbox}", s.getSandbox)
	s.mux.HandleFunc("GET /v1/sandboxes/{sandbox}/evidence", s.getEvidence)
	s.mux.HandleFunc("POST /v1/apply", s.apply)
	s.mux.HandleFunc("GET /v1/bundles/{digest}", s.getBundle)
}

// actor is the authenticated identity a request acts as. The sim deployment
// trusts a header the way a real deployment trusts its auth layer.
func actor(r *http.Request) string {
	if a := r.Header.Get("X-Cloudbox-User"); a != "" {
		return a
	}
	return "anonymous"
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
		body := map[string]any{"error": ce.Message}
		if len(ce.Findings) > 0 {
			body["findings"] = ce.Findings
		}
		writeJSON(w, ce.Status, body)
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

func (s *Server) updateContract(w http.ResponseWriter, r *http.Request) {
	// Strict decoding is the C3 mechanism: the four contract kinds are the
	// complete schema, so an unknown kind is rejected structurally.
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var contract core.Contract
	if err := dec.Decode(&contract); err != nil {
		writeJSON(w, 422, map[string]string{"error": fmt.Sprintf(
			"contract rejected (%v): environment variance is limited to secret names, ingress hostnames, the egress allowlist, and internal application dependencies — anything else is out of contract by design; the path is a change to the product spec, not an overlay or templating mechanism",
			err)})
		return
	}
	if err := s.core.UpdateContract(r.PathValue("app"), contract); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, contract)
}

func (s *Server) setSecretValue(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Environment string `json:"environment"`
		Name        string `json:"name"`
		Value       string `json:"value"` // write-only: recorded as present, never stored
	}](w, r)
	if !ok {
		return
	}
	if err := s.core.SetSecretValue(r.PathValue("app"), req.Environment, req.Name); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "value recorded", "environment": req.Environment})
}

func (s *Server) createSandbox(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		App string `json:"app"`
	}](w, r)
	if !ok {
		return
	}
	sb, err := s.core.CreateSandbox(req.App, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, sb)
}

func (s *Server) getSandbox(w http.ResponseWriter, r *http.Request) {
	sb, err := s.core.GetSandbox(r.PathValue("sandbox"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, sb)
}

func (s *Server) getEvidence(w http.ResponseWriter, r *http.Request) {
	ev, err := s.core.GetEvidence(r.PathValue("sandbox"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, ev)
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		App       string `json:"app"`
		Sandbox   string `json:"sandbox"`
		Manifests string `json:"manifests"`
	}](w, r)
	if !ok {
		return
	}
	result, err := s.core.Apply(req.App, req.Sandbox, actor(r), req.Manifests)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) getBundle(w http.ResponseWriter, r *http.Request) {
	b, err := s.core.GetBundle(r.PathValue("digest"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"digest":     b.Digest,
		"manifests":  b.ManifestYAML,
		"transforms": b.Transforms,
		"findings":   b.Findings,
	})
}
