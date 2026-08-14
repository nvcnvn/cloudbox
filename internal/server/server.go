// Package server is the cloudboxd HTTP surface. Everything the CLI and SCM
// integration do goes through here (ADR 0004: server-side authority).
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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
	s.core = core.New(world, world.Now)
}

func (s *Server) routes() {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /healthz", s.healthz)

	// /simctl/* is the sim driver's test-arrangement surface (ADR 0007):
	// conjure clusters, advance the clock, arrange out-of-band state the way
	// Given-steps need. Only wired when the sim driver is in play.
	s.mux.HandleFunc("POST /simctl/reset", s.simReset)
	s.mux.HandleFunc("POST /simctl/clusters", s.simCreateCluster)
	s.mux.HandleFunc("POST /simctl/clusters/{cluster}/objects", s.simApplyRaw)
	s.mux.HandleFunc("GET /simctl/clusters/{cluster}/objects", s.simGetRaw)
	s.mux.HandleFunc("POST /simctl/clusters/{cluster}/components", s.simSetComponents)
	s.mux.HandleFunc("POST /simctl/advance-time", s.simAdvanceTime)
	s.mux.HandleFunc("POST /simctl/hold-seal", s.simHoldSeal)
	s.mux.HandleFunc("POST /simctl/sandboxes/{sandbox}/complete-seal", s.simCompleteSeal)
	s.mux.HandleFunc("POST /simctl/sandboxes/{sandbox}/egress-attempts", s.simAttemptEgress)
	s.mux.HandleFunc("POST /simctl/oom-under-squeeze", s.simMarkOOM)

	s.mux.HandleFunc("POST /v1/setup", s.setup)
	s.mux.HandleFunc("POST /v1/clusters/register", s.registerCluster)
	s.mux.HandleFunc("GET /v1/clusters/{cluster}/crds", s.listCRDs)
	s.mux.HandleFunc("POST /v1/applications", s.createApplication)
	s.mux.HandleFunc("GET /v1/applications/{app}", s.getApplication)
	s.mux.HandleFunc("PUT /v1/applications/{app}/contract", s.updateContract)
	s.mux.HandleFunc("PUT /v1/applications/{app}/allowlist", s.updateAllowlist)
	s.mux.HandleFunc("PUT /v1/applications/{app}/secret-values", s.setSecretValue)
	s.mux.HandleFunc("GET /v1/applications/{app}/substrate-lockfile", s.getLockfile)
	s.mux.HandleFunc("POST /v1/sandboxes", s.createSandbox)
	s.mux.HandleFunc("GET /v1/sandboxes/{sandbox}", s.getSandbox)
	s.mux.HandleFunc("DELETE /v1/sandboxes/{sandbox}", s.destroySandbox)
	s.mux.HandleFunc("GET /v1/sandboxes/{sandbox}/workloads", s.sandboxWorkloads)
	s.mux.HandleFunc("GET /v1/sandboxes/{sandbox}/evidence", s.getEvidence)
	s.mux.HandleFunc("POST /v1/sandboxes/{sandbox}/evidence/override-substrate", s.overrideSubstrate)
	s.mux.HandleFunc("POST /v1/sandboxes/{sandbox}/allowlist-requests", s.requestAllowlist)
	s.mux.HandleFunc("GET /v1/sandboxes/{sandbox}/explain", s.explain)
	s.mux.HandleFunc("GET /v1/sandboxes/{sandbox}/workloads/{workload}/logs", s.workloadLogs)
	s.mux.HandleFunc("POST /v1/sandboxes/{sandbox}/exec", s.execInWorkload)
	s.mux.HandleFunc("POST /v1/sandboxes/{sandbox}/port-forward", s.portForward)
	s.mux.HandleFunc("POST /v1/apply", s.apply)
	s.mux.HandleFunc("GET /v1/bundles/{digest}", s.getBundle)
	s.mux.HandleFunc("POST /v1/scm/events", s.scmEvent)
	s.mux.HandleFunc("GET /v1/containment-statement", s.containment)
	s.mux.HandleFunc("GET /v1/audit", s.auditLog)

	// Evidence, diff, witnessed tests, merge binding, checks (G2/G7/G13/X4).
	s.mux.HandleFunc("POST /simctl/production/{app}", s.simSetProduction)
	s.mux.HandleFunc("GET /v1/applications/{app}/diff/{digest}", s.diff)
	s.mux.HandleFunc("POST /v1/sandboxes/{sandbox}/test", s.runTests)
	s.mux.HandleFunc("POST /v1/sandboxes/{sandbox}/test-results", s.assertTestResults)
	s.mux.HandleFunc("POST /v1/sandboxes/{sandbox}/datastores", s.setDatastore)
	s.mux.HandleFunc("GET /v1/applications/{app}/prs/{pr}/merge-result", s.mergeResult)
	s.mux.HandleFunc("GET /v1/scm/prs/{app}/{pr}/checks", s.listChecks)
	s.mux.HandleFunc("POST /simctl/scm/prs/{app}/{pr}/checks", s.simPipelineCheck)
	s.mux.HandleFunc("POST /v1/evidence-checks", s.postEvidenceCheck)

	// Promotion (G1/G4/G5/G8/G9/G11/G12).
	s.mux.HandleFunc("POST /v1/promotions", s.openPromotion)
	s.mux.HandleFunc("GET /v1/promotions/{id}", s.getPromotion)
	s.mux.HandleFunc("GET /v1/promotions", s.listPromotions)
	s.mux.HandleFunc("POST /v1/promotions/{id}/approve", s.approvePromotion)
	s.mux.HandleFunc("POST /v1/promotions/{id}/apply", s.applyPromotion)
	s.mux.HandleFunc("POST /v1/promotions/rollback", s.openRollback)
	s.mux.HandleFunc("POST /v1/break-glass", s.grantBreakGlass)
	s.mux.HandleFunc("GET /v1/applications/{app}/credentials", s.credentialsReport)
	s.mux.HandleFunc("GET /v1/applications/{app}/production-status", s.productionStatus)
	s.mux.HandleFunc("POST /simctl/gitops/{app}/sync", s.simGitOpsSync)
	s.mux.HandleFunc("POST /simctl/gitops/{app}/fail-next-sync", s.simFailNextSync)
	s.mux.HandleFunc("POST /simctl/audit-sink", s.simAuditSink)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// actor is the authenticated identity a request acts as. The sim deployment
// trusts a header the way a real deployment trusts its auth layer.
func actor(r *http.Request) string {
	if a := r.Header.Get("X-Cloudbox-User"); a != "" {
		return a
	}
	return "anonymous"
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
	s.world.CreateCluster(req.Name, enforcing, false)
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

func (s *Server) simSetComponents(w http.ResponseWriter, r *http.Request) {
	cl, ok := s.world.Cluster(r.PathValue("cluster"))
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "cluster not known"})
		return
	}
	req, decoded := decode[struct {
		KubernetesMinor string              `json:"kubernetesMinor"`
		Components      []cluster.Component `json:"components"`
	}](w, r)
	if !decoded {
		return
	}
	cl.SetComponents(req.KubernetesMinor, req.Components)
	writeJSON(w, 200, map[string]string{"status": "components set"})
}

func (s *Server) simAdvanceTime(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Seconds int64 `json:"seconds"`
	}](w, r)
	if !ok {
		return
	}
	now := s.world.Advance(time.Duration(req.Seconds) * time.Second)
	s.core.Tick()
	writeJSON(w, 200, map[string]string{"now": now.Format(time.RFC3339)})
}

func (s *Server) simHoldSeal(w http.ResponseWriter, _ *http.Request) {
	s.core.HoldNextSeal()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) simCompleteSeal(w http.ResponseWriter, r *http.Request) {
	if err := s.core.CompleteSeal(r.PathValue("sandbox")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "sealed"})
}

func (s *Server) simAttemptEgress(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Workload    string `json:"workload"`
		Destination string `json:"destination"`
	}](w, r)
	if !ok {
		return
	}
	result, err := s.core.AttemptEgress(r.PathValue("sandbox"), req.Workload, req.Destination)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) simMarkOOM(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Workload string `json:"workload"`
	}](w, r)
	if !ok {
		return
	}
	s.world.MarkOOMUnderSqueeze(req.Workload)
	w.WriteHeader(http.StatusNoContent)
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

func (s *Server) registerCluster(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Cluster string `json:"cluster"`
		Role    string `json:"role"`
	}](w, r)
	if !ok {
		return
	}
	if err := s.core.RegisterCluster(req.Cluster, req.Role); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "registered", "role": req.Role})
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

func (s *Server) updateAllowlist(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Allowlist []string `json:"allowlist"`
	}](w, r)
	if !ok {
		return
	}
	if err := s.core.UpdateAllowlist(r.PathValue("app"), actor(r), req.Allowlist); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"allowlist": req.Allowlist})
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

func (s *Server) getLockfile(w http.ResponseWriter, r *http.Request) {
	lf, err := s.core.SubstrateLockfile(r.PathValue("app"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, lf)
}

func (s *Server) createSandbox(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		App        string `json:"app"`
		Local      bool   `json:"local"`
		TTLSeconds int64  `json:"ttlSeconds"`
	}](w, r)
	if !ok {
		return
	}
	sb, err := s.core.CreateSandbox(req.App, actor(r), core.CreateSandboxOptions{
		Local: req.Local, TTLSeconds: req.TTLSeconds,
	})
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

func (s *Server) destroySandbox(w http.ResponseWriter, r *http.Request) {
	if err := s.core.DestroySandbox(r.PathValue("sandbox"), actor(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sandboxWorkloads(w http.ResponseWriter, r *http.Request) {
	workloads, err := s.core.SandboxWorkloads(r.PathValue("sandbox"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"workloads": workloads})
}

func (s *Server) getEvidence(w http.ResponseWriter, r *http.Request) {
	ev, err := s.core.GetEvidence(r.PathValue("sandbox"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, ev)
}

func (s *Server) overrideSubstrate(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Reason string `json:"reason"`
	}](w, r)
	if !ok {
		return
	}
	if err := s.core.OverrideSubstrateMismatch(r.PathValue("sandbox"), actor(r), req.Reason); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "override recorded"})
}

func (s *Server) requestAllowlist(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		FQDN string `json:"fqdn"`
	}](w, r)
	if !ok {
		return
	}
	err := s.core.RequestAllowlistChange(r.PathValue("sandbox"), actor(r), req.FQDN)
	writeErr(w, err)
}

func (s *Server) explain(w http.ResponseWriter, r *http.Request) {
	out, err := s.core.Explain(r.PathValue("sandbox"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) workloadLogs(w http.ResponseWriter, r *http.Request) {
	out, err := s.core.Logs(r.PathValue("sandbox"), r.PathValue("workload"), actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"logs": out})
}

func (s *Server) execInWorkload(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Workload string `json:"workload"`
		Command  string `json:"command"`
	}](w, r)
	if !ok {
		return
	}
	out, err := s.core.Exec(r.PathValue("sandbox"), req.Workload, req.Command, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"output": out})
}

func (s *Server) portForward(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Workload string `json:"workload"`
		Port     int    `json:"port"`
	}](w, r)
	if !ok {
		return
	}
	out, err := s.core.PortForward(r.PathValue("sandbox"), req.Workload, req.Port, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		App          string `json:"app"`
		Sandbox      string `json:"sandbox"`
		Manifests    string `json:"manifests"`
		CapacityMode string `json:"capacityMode"`
		RecordEgress bool   `json:"recordEgress"`
	}](w, r)
	if !ok {
		return
	}
	result, err := s.core.Apply(req.App, req.Sandbox, actor(r), req.Manifests, core.ApplyOptions{
		CapacityMode: req.CapacityMode, RecordEgress: req.RecordEgress,
	})
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

func (s *Server) scmEvent(w http.ResponseWriter, r *http.Request) {
	ev, ok := decode[core.SCMEvent](w, r)
	if !ok {
		return
	}
	sb, err := s.core.HandleSCMEvent(ev)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, sb)
}

func (s *Server) containment(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.core.Containment())
}

func (s *Server) auditLog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"entries": s.core.AuditLog()})
}

func (s *Server) simSetProduction(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Manifests string `json:"manifests"`
	}](w, r)
	if !ok {
		return
	}
	if err := s.core.SetProductionState(r.PathValue("app"), actor(r), req.Manifests); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "production state set"})
}

func (s *Server) diff(w http.ResponseWriter, r *http.Request) {
	lines, err := s.core.DiffAgainstProduction(r.PathValue("app"), r.PathValue("digest"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"diff": lines})
}

func (s *Server) runTests(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Suite string `json:"suite"`
	}](w, r)
	if !ok {
		return
	}
	run, err := s.core.RunTests(r.PathValue("sandbox"), req.Suite, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, run)
}

func (s *Server) assertTestResults(w http.ResponseWriter, r *http.Request) {
	writeErr(w, s.core.AssertTestResults(r.PathValue("sandbox")))
}

func (s *Server) setDatastore(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Name     string `json:"name"`
		Fidelity string `json:"fidelity"`
	}](w, r)
	if !ok {
		return
	}
	if err := s.core.SetDatastoreFidelity(r.PathValue("sandbox"), req.Name, req.Fidelity); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "datastore recorded", "fidelity": req.Fidelity})
}

func (s *Server) mergeResult(w http.ResponseWriter, r *http.Request) {
	result, err := s.core.MergeResultFor(r.PathValue("app"), r.PathValue("pr"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) listChecks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"checks": s.core.ChecksFor(r.PathValue("app"), r.PathValue("pr")),
	})
}

func (s *Server) simPipelineCheck(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
	}](w, r)
	if !ok {
		return
	}
	check := s.core.SimulatePipelineCheck(r.PathValue("app"), r.PathValue("pr"), req.Name, req.Summary)
	writeJSON(w, 201, check)
}

func (s *Server) postEvidenceCheck(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Sandbox string `json:"sandbox"`
		PR      string `json:"pr"`
	}](w, r)
	if !ok {
		return
	}
	check, err := s.core.PostEvidenceCheck(req.Sandbox, req.PR)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, check)
}

func (s *Server) openPromotion(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Sandbox string `json:"sandbox"`
	}](w, r)
	if !ok {
		return
	}
	p, err := s.core.OpenPromotion(req.Sandbox, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, p)
}

func (s *Server) getPromotion(w http.ResponseWriter, r *http.Request) {
	p, err := s.core.GetPromotion(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) listPromotions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"promotions": s.core.ListPromotions(r.URL.Query().Get("app"))})
}

func (s *Server) approvePromotion(w http.ResponseWriter, r *http.Request) {
	p, err := s.core.Approve(r.PathValue("id"), actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) applyPromotion(w http.ResponseWriter, r *http.Request) {
	p, err := s.core.ExecuteApply(r.PathValue("id"), actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) openRollback(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		App    string `json:"app"`
		Digest string `json:"digest"`
	}](w, r)
	if !ok {
		return
	}
	p, err := s.core.OpenRollback(req.App, req.Digest, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, p)
}

func (s *Server) grantBreakGlass(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		App string `json:"app"`
	}](w, r)
	if !ok {
		return
	}
	grant, err := s.core.GrantBreakGlass(req.App, actor(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, grant)
}

func (s *Server) credentialsReport(w http.ResponseWriter, r *http.Request) {
	report, err := s.core.CredentialsReport(r.PathValue("app"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, report)
}

func (s *Server) productionStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.core.ProductionStatus(r.PathValue("app"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, status)
}

func (s *Server) simGitOpsSync(w http.ResponseWriter, r *http.Request) {
	p, err := s.core.GitOpsSync(r.PathValue("app"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) simFailNextSync(w http.ResponseWriter, r *http.Request) {
	s.core.FailNextSync(r.PathValue("app"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) simAuditSink(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Available bool `json:"available"`
	}](w, r)
	if !ok {
		return
	}
	s.core.SetAuditSink(req.Available)
	w.WriteHeader(http.StatusNoContent)
}
