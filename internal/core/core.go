// Package core is the control plane: all validation, bundling, evidence
// gathering, signing, and enforcement live here, server-side (ADR 0004).
package core

import (
	"fmt"
	"sync"

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
// plus the alias hostname bundles use to reach it.
type Dependency struct {
	App   string `json:"app"`
	Alias string `json:"alias,omitempty"`
}

// Application is the policy boundary (§5).
type Application struct {
	Name      string   `json:"name"`
	Owners    []string `json:"owners"`
	Approvers []string `json:"approvers"`
	Level     string   `json:"level"` // L1..L4
	Contract  Contract `json:"contract"`
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

	apps       map[string]*Application
	installed  map[string]bool // clusters where setup ran
	bundles    map[string]*Bundle
	sandboxes  map[string]*Sandbox
	evidence   map[string]*Evidence // by sandbox name
	sandboxSeq int
}

func New(driver cluster.Driver) *Core {
	return &Core{
		driver:    driver,
		apps:      map[string]*Application{},
		installed: map[string]bool{},
		bundles:   map[string]*Bundle{},
		sandboxes: map[string]*Sandbox{},
		evidence:  map[string]*Evidence{},
	}
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
