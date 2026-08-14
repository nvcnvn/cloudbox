// Package core is the control plane: all validation, bundling, evidence
// gathering, signing, and enforcement live here, server-side (ADR 0004).
package core

import (
	"fmt"
	"sync"
	"time"

	"cloudbox/internal/cluster"
)

// APIGroup is the one API group all five product CRDs live under (CP1).
const APIGroup = "cloudbox.dev"

// ProductCRDs is the complete CRD surface: exactly five, never more (CP1).
func ProductCRDs() []cluster.CRD {
	kinds := []string{"Application", "Sandbox", "Bundle", "PromotionRequest", "ClusterRegistry"}
	names := []string{"applications", "sandboxes", "bundles", "promotionrequests", "clusterregistries"}
	crds := make([]cluster.CRD, len(kinds))
	for i, kind := range kinds {
		crds[i] = cluster.CRD{Name: names[i] + "." + APIGroup, Group: APIGroup, Kind: kind}
	}
	return crds
}

// Contract is the boundary contract (C1): the complete, finite set of values
// allowed to differ per environment. Nothing else varies (C3).
type Contract struct {
	SecretNames      []string     `json:"secretNames"`
	IngressHostnames []string     `json:"ingressHostnames"`
	EgressAllowlist  []string     `json:"egressAllowlist"`
	Dependencies     []Dependency `json:"dependencies"`
}

// Dependency is an internal application dependency (C1): a target Application
// plus the alias hostname bundles use to reach it. In v1 a dependency may be
// satisfied by an allowlisted stub endpoint, recorded as stubbed in evidence
// (S8 v1 alternative).
type Dependency struct {
	App          string `json:"app"`
	Alias        string `json:"alias,omitempty"`
	StubEndpoint string `json:"stubEndpoint,omitempty"`
}

// Policies are per-application operational policy (S5, S7, G4, D3, X4).
type Policies struct {
	// CPUQuotaPerSandbox caps total CPU requests (in cores) a sandbox may
	// schedule after transforms; 0 means unlimited (S5).
	CPUQuotaPerSandbox float64 `json:"cpuQuotaPerSandbox,omitempty"`
	// IdleExpirySeconds destroys a sandbox with no activity for this long;
	// 0 disables idle expiry (S5).
	IdleExpirySeconds int64 `json:"idleExpirySeconds,omitempty"`
	// MinFidelity is the minimum data fidelity for valid evidence (D3).
	MinFidelity string `json:"minFidelity,omitempty"`
	// MinFidelityForMigrations conditionally raises the floor when a bundle
	// contains a migration (D3).
	MinFidelityForMigrations string `json:"minFidelityForMigrations,omitempty"`
	// MinWitnessedEvents is the witnessed-activity floor for a valid evidence
	// check (X4/G13).
	MinWitnessedEvents int `json:"minWitnessedEvents,omitempty"`
	// RequiredApprovals is the approver count a promotion needs (G4);
	// 0 means 1.
	RequiredApprovals int `json:"requiredApprovals,omitempty"`
}

// TestSuite is the application's declared suite the test command runs (X4).
type TestSuite struct {
	Name  string `json:"name"`
	Tests int    `json:"tests"`
}

// Application is the policy boundary (§5).
type Application struct {
	Name           string     `json:"name"`
	Owners         []string   `json:"owners"`
	Approvers      []string   `json:"approvers"`
	Level          string     `json:"level"` // L1..L4
	Contract       Contract   `json:"contract"`
	Policies       Policies   `json:"policies"`
	SCMIntegration bool       `json:"scmIntegration"` // enables PR-bound sandboxes (S6)
	TestSuite      *TestSuite `json:"testSuite,omitempty"`
	// SandboxCluster optionally pins this application's sandboxes to one
	// registered cluster (CP3 topologies).
	SandboxCluster string `json:"sandboxCluster,omitempty"`
	// GitOpsPath is the declared production path write-back commits to (G9).
	GitOpsPath string `json:"gitOpsPath,omitempty"`
	// BreakGlassRole names the emergency identities allowed break-glass
	// access in strict mode (G12); strict mode refuses setup without one.
	BreakGlassRole []string `json:"breakGlassRole,omitempty"`
	// Datastores the application declares (D1).
	Datastores []DeclaredDatastore `json:"datastores,omitempty"`
	// RealDataEnabled/RealDataForAgents gate masked-snapshot and live-clone:
	// admin-enabled per application, never default (D8).
	RealDataEnabled   bool `json:"realDataEnabled,omitempty"`
	RealDataForAgents bool `json:"realDataForAgents,omitempty"`
}

// Error is a user-facing failure with an HTTP-ish status. Intake rejections
// carry the full finding list so every rejection names the violating manifest
// and the fix (B3).
type Error struct {
	Status   int
	Message  string
	Findings []Finding
}

func (e *Error) Error() string { return e.Message }

func errf(status int, format string, args ...any) *Error {
	return &Error{Status: status, Message: fmt.Sprintf(format, args...)}
}

// Core is the whole control plane behind the HTTP surface.
type Core struct {
	mu     sync.Mutex
	driver cluster.Driver
	now    func() time.Time

	apps      map[string]*Application
	installed map[string]bool // clusters where setup ran
	// roles holds registered cluster roles; one cluster may hold several
	// (shared-cluster topology, CP3): "sandbox" | "production".
	roles map[string]map[string]bool
	bundles   map[string]*Bundle
	sandboxes map[string]*Sandbox
	evidence  map[string]*Evidence // by sandbox name
	// secretValues records presence only, by app → environment → secret name:
	// the values themselves are write-only and never stored (C1).
	secretValues map[string]map[string]map[string]bool
	production   map[string]*ProductionState // what the team's CD runs (G2)
	mergeResults map[string]*MergeResult     // app/pr → merge-time binding (G7)
	prChecks     map[string][]PRCheck        // app/pr → posted status checks (G13/CP4)
	promotions   map[string]*Promotion
	gitops       map[string]*gitopsCommit       // app → last write-back commit (G9)
	history      map[string][]*HistoryEntry     // app → applied production bundles (G11)
	promoted     map[string]*PromotedState      // app → live promoted state (G12)
	breakGlass   map[string]map[string]time.Time // app → actor → expiry (G12)
	failNextSync map[string]bool
	databases    map[string]*SimDatabase // app/ds → production database (sim)
	profiles     map[string]*DataProfile // app/ds → data profile lockfile (D1)
	audit          []AuditEntry
	auditAvailable bool
	promotionSeq   int
	sandboxSeq     int
	holdSeal       bool // sim arrangement: leave the next sandbox provisioning (N1)
}

// AuditEntry is one synchronous audit record (G5, G12, P2).
type AuditEntry struct {
	At      time.Time `json:"at"`
	Actor   string    `json:"actor"`
	Action  string    `json:"action"`
	Subject string    `json:"subject"`
	Detail  string    `json:"detail,omitempty"`
}

func New(driver cluster.Driver, now func() time.Time) *Core {
	return &Core{
		driver:       driver,
		now:          now,
		apps:         map[string]*Application{},
		installed:    map[string]bool{},
		roles:        map[string]map[string]bool{},
		bundles:      map[string]*Bundle{},
		sandboxes:    map[string]*Sandbox{},
		evidence:     map[string]*Evidence{},
		secretValues: map[string]map[string]map[string]bool{},
		production:   map[string]*ProductionState{},
		mergeResults: map[string]*MergeResult{},
		prChecks:     map[string][]PRCheck{},
		promotions:   map[string]*Promotion{},
		gitops:       map[string]*gitopsCommit{},
		history:      map[string][]*HistoryEntry{},
		promoted:     map[string]*PromotedState{},
		breakGlass:   map[string]map[string]time.Time{},
		failNextSync: map[string]bool{},
		databases:    map[string]*SimDatabase{},
		profiles:     map[string]*DataProfile{},
		auditAvailable: true,
	}
}

// RegisterCluster records a cluster's role in the ClusterRegistry (CP3).
func (c *Core) RegisterCluster(name, role string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.driver.Cluster(name); !ok {
		return errf(404, "cluster %q is not known", name)
	}
	if c.roles[name] == nil {
		c.roles[name] = map[string]bool{}
	}
	c.roles[name][role] = true
	return nil
}

// HoldNextSeal is sim arrangement for N1: the next sandbox stays provisioning
// until its seal is explicitly completed.
func (c *Core) HoldNextSeal() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.holdSeal = true
}

// recordAudit appends one synchronous audit record. Callers hold c.mu.
func (c *Core) recordAudit(actor, action, subject, detail string) {
	c.audit = append(c.audit, AuditEntry{
		At: c.now(), Actor: actor, Action: action, Subject: subject, Detail: detail,
	})
}

// AuditLog returns the audit trail.
func (c *Core) AuditLog() []AuditEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]AuditEntry{}, c.audit...)
}

// Setup installs the controller and the five product CRDs on a cluster.
func (c *Core) Setup(clusterName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cl, ok := c.driver.Cluster(clusterName)
	if !ok {
		return errf(404, "cluster %q is not known", clusterName)
	}
	cl.InstallCRDs(ProductCRDs())
	c.installed[clusterName] = true
	return nil
}

// InstalledCRDs lists what setup put on the cluster.
func (c *Core) InstalledCRDs(clusterName string) ([]cluster.CRD, error) {
	cl, ok := c.driver.Cluster(clusterName)
	if !ok {
		return nil, errf(404, "cluster %q is not known", clusterName)
	}
	return cl.ListCRDs(), nil
}

// CreateApplication admits an Application, validating its dependency graph
// across Applications (CP1): every declared dependency must name one that
// exists.
func (c *Core) CreateApplication(app *Application) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if app.Name == "" {
		return errf(422, "application name is required")
	}
	if _, exists := c.apps[app.Name]; exists {
		return errf(409, "application %q already exists", app.Name)
	}
	for _, dep := range app.Contract.Dependencies {
		if _, ok := c.apps[dep.App]; !ok {
			return errf(422,
				"dangling dependency reference: application %q declares a dependency on %q, which does not exist",
				app.Name, dep.App)
		}
	}
	if app.Level == "" {
		app.Level = "L1"
	}
	if app.Level == "L4" && len(app.BreakGlassRole) == 0 {
		// G12: an escape hatch that exists is auditable; one that doesn't
		// gets improvised.
		return errf(422,
			"setup failed: strict mode requires a configured break-glass role — name the emergency identities before enabling L4")
	}
	c.apps[app.Name] = app
	return nil
}

// GetApplication returns an admitted application.
func (c *Core) GetApplication(name string) (*Application, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[name]
	if !ok {
		return nil, errf(404, "application %q is not known", name)
	}
	return app, nil
}

// UpdateContract replaces an application's boundary contract (C1). The four
// declared kinds are the complete set; the HTTP layer rejects anything else
// structurally (C3).
func (c *Core) UpdateContract(appName string, contract Contract) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[appName]
	if !ok {
		return errf(404, "application %q is not known", appName)
	}
	for _, dep := range contract.Dependencies {
		if _, ok := c.apps[dep.App]; !ok {
			return errf(422,
				"dangling dependency reference: contract declares a dependency on %q, which does not exist",
				dep.App)
		}
	}
	app.Contract = contract
	return nil
}

// SetSecretValue records that a declared secret has a value for one
// environment. The value itself is write-only: values are supplied per
// environment and never live inside bundles or contract records (C1).
func (c *Core) SetSecretValue(appName, environment, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[appName]
	if !ok {
		return errf(404, "application %q is not known", appName)
	}
	declared := false
	for _, s := range app.Contract.SecretNames {
		if s == name {
			declared = true
		}
	}
	if !declared {
		return errf(422, "secret %q is not declared in the boundary contract", name)
	}
	if c.secretValues[appName] == nil {
		c.secretValues[appName] = map[string]map[string]bool{}
	}
	if c.secretValues[appName][environment] == nil {
		c.secretValues[appName][environment] = map[string]bool{}
	}
	c.secretValues[appName][environment][name] = true
	return nil
}

// hasSecretValue reports whether a declared secret has a value in an
// environment. Callers hold c.mu.
func (c *Core) hasSecretValue(appName, environment, name string) bool {
	return c.secretValues[appName][environment][name]
}
