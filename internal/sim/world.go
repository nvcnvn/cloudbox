// Package sim is the simulated cluster driver (ADR 0007): an in-process model
// of the Kubernetes semantics the specs exercise. The acceptance suite boots
// cloudboxd against it; the kube driver is the production path behind the same
// interfaces.
package sim

import (
	"strings"
	"sync"
	"time"

	"cloudbox/internal/cluster"
)

// World holds every simulated cluster and the simulated clock for one
// cloudboxd process.
type World struct {
	mu       sync.Mutex
	clusters map[string]*SimCluster
	now      time.Time
	// oomUnderSqueeze names workloads that cannot survive the squeezed
	// capacity transform (arranged by the suite to model memory-tight apps).
	oomUnderSqueeze map[string]bool
	// failingMigrations names migration Jobs whose chain fails (D4).
	failingMigrations map[string]bool
}

func NewWorld() *World {
	return &World{
		clusters:          map[string]*SimCluster{},
		now:               time.Now(),
		oomUnderSqueeze:   map[string]bool{},
		failingMigrations: map[string]bool{},
	}
}

// Now is the simulated clock; the suite advances it to model TTLs and soak.
func (w *World) Now() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.now
}

func (w *World) Advance(d time.Duration) time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.now = w.now.Add(d)
	return w.now
}

// Cluster implements cluster.Driver.
func (w *World) Cluster(name string) (cluster.Cluster, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, ok := w.clusters[name]
	return c, ok
}

// CreateCluster is sim-only arrangement. Idempotent: re-declaring an existing
// cluster keeps its state so page objects can call it freely.
func (w *World) CreateCluster(name string, enforcing bool, userControlled bool) *SimCluster {
	w.mu.Lock()
	defer w.mu.Unlock()
	if c, ok := w.clusters[name]; ok {
		return c
	}
	c := &SimCluster{
		world:          w,
		name:           name,
		enforcesNetPol: enforcing,
		userControlled: userControlled,
		k8sMinor:       "1.31",
		rawObjects:     map[string]cluster.Object{},
		namespaces:     map[string]*simNamespace{},
	}
	w.clusters[name] = c
	return c
}

// NewCluster is the control plane's local-provisioning hook (S3): same
// factory as CreateCluster, typed to the driver interface.
func (w *World) NewCluster(name string, enforcing, userControlled bool) cluster.Cluster {
	return w.CreateCluster(name, enforcing, userControlled)
}

// MarkOOMUnderSqueeze arranges that a workload of this name is OOM-killed when
// admitted under the squeezed transform.
func (w *World) MarkOOMUnderSqueeze(workload string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.oomUnderSqueeze[workload] = true
}

// OOMsUnderSqueeze reports the arrangement to the control plane's admission
// path.
func (w *World) OOMsUnderSqueeze(workload string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.oomUnderSqueeze[workload]
}

// MarkMigrationFailing arranges that a migration Job of this name fails when
// its chain runs (D4).
func (w *World) MarkMigrationFailing(workload string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failingMigrations[workload] = true
}

// FailsMigration reports the arrangement to the control plane.
func (w *World) FailsMigration(workload string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.failingMigrations[workload]
}

type simNamespace struct {
	sealed    bool
	allowlist []string
	workloads []cluster.Workload
}

// SimCluster is one simulated cluster.
type SimCluster struct {
	mu             sync.Mutex
	world          *World
	name           string
	enforcesNetPol bool
	userControlled bool
	k8sMinor       string
	components     []cluster.Component
	crds           []cluster.CRD
	rawObjects     map[string]cluster.Object
	namespaces     map[string]*simNamespace
}

func rawKey(ns, kind, name string) string { return ns + "/" + kind + "/" + name }

func (c *SimCluster) Name() string         { return c.name }
func (c *SimCluster) UserControlled() bool { return c.userControlled }

func (c *SimCluster) InstallCRDs(crds []cluster.CRD) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.crds = append([]cluster.CRD{}, crds...)
}

func (c *SimCluster) ListCRDs() []cluster.CRD {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]cluster.CRD{}, c.crds...)
}

func (c *SimCluster) ApplyRaw(obj cluster.Object) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rawObjects[rawKey(obj.Namespace, obj.Kind, obj.Name)] = obj
}

func (c *SimCluster) GetRaw(ns, kind, name string) (cluster.Object, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	obj, ok := c.rawObjects[rawKey(ns, kind, name)]
	return obj, ok
}

func (c *SimCluster) KubernetesMinor() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.k8sMinor
}

func (c *SimCluster) Components() []cluster.Component {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]cluster.Component{}, c.components...)
}

func (c *SimCluster) SetComponents(k8sMinor string, comps []cluster.Component) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if k8sMinor != "" {
		c.k8sMinor = k8sMinor
	}
	c.components = append([]cluster.Component{}, comps...)
}

func (c *SimCluster) EnsureNamespace(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.namespaces[name]; !ok {
		c.namespaces[name] = &simNamespace{}
	}
}

func (c *SimCluster) DeleteNamespace(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.namespaces, name)
}

func (c *SimCluster) SealNamespace(name string, allowlist []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ns, ok := c.namespaces[name]
	if !ok {
		ns = &simNamespace{}
		c.namespaces[name] = ns
	}
	// The policy objects are always accepted — a CNI that does not enforce
	// them fails silently, which is exactly what ProbeEnforcement catches.
	ns.sealed = true
	ns.allowlist = append([]string{}, allowlist...)
}

func (c *SimCluster) ProbeEnforcement() bool {
	// Canary probe: seal a scratch namespace, attempt a denied connection,
	// verify it is actually denied.
	const probeNS = "cloudbox-probe"
	c.EnsureNamespace(probeNS)
	c.SealNamespace(probeNS, nil)
	result := c.AttemptEgress(probeNS, "enforcement-probe.invalid")
	c.DeleteNamespace(probeNS)
	return !result.Allowed
}

func (c *SimCluster) AttemptEgress(namespace, destination string) cluster.EgressResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	ns, ok := c.namespaces[namespace]
	if !ok {
		return cluster.EgressResult{Allowed: false, Via: "denied"}
	}
	if !ns.sealed || !c.enforcesNetPol {
		// No seal, or a CNI that accepts-and-ignores NetworkPolicy: nothing
		// filters the connection. This is N7's silent failure mode.
		return cluster.EgressResult{Allowed: true, Via: "unfiltered"}
	}
	if destination == "cluster-dns" {
		return cluster.EgressResult{Allowed: true, Via: "cluster-dns"}
	}
	if !strings.Contains(destination, ".") {
		// Same-namespace short name: reachable if such a workload exists.
		for _, w := range ns.workloads {
			if w.Name == destination {
				return cluster.EgressResult{Allowed: true, Via: "in-sandbox"}
			}
		}
		return cluster.EgressResult{Allowed: false, Via: "denied"}
	}
	for _, fqdn := range ns.allowlist {
		if fqdn == destination {
			return cluster.EgressResult{Allowed: true, Via: "egress-proxy"}
		}
	}
	return cluster.EgressResult{Allowed: false, Via: "denied"}
}

func (c *SimCluster) AddWorkload(namespace string, w cluster.Workload) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ns, ok := c.namespaces[namespace]
	if !ok {
		ns = &simNamespace{}
		c.namespaces[namespace] = ns
	}
	for i := range ns.workloads {
		if ns.workloads[i].Name == w.Name {
			ns.workloads[i] = w
			return
		}
	}
	ns.workloads = append(ns.workloads, w)
}

func (c *SimCluster) Workloads(namespace string) []cluster.Workload {
	c.mu.Lock()
	defer c.mu.Unlock()
	ns, ok := c.namespaces[namespace]
	if !ok {
		return nil
	}
	return append([]cluster.Workload{}, ns.workloads...)
}
