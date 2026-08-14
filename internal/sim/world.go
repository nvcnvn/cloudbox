// Package sim is the simulated cluster driver (ADR 0007): an in-process model
// of the Kubernetes semantics the specs exercise. The acceptance suite boots
// cloudboxd against it; the kube driver is the production path behind the same
// interfaces.
package sim

import (
	"sync"

	"cloudbox/internal/cluster"
)

// World holds every simulated cluster for one cloudboxd process.
type World struct {
	mu       sync.Mutex
	clusters map[string]*SimCluster
}

func NewWorld() *World {
	return &World{clusters: map[string]*SimCluster{}}
}

// Cluster implements cluster.Driver.
func (w *World) Cluster(name string) (cluster.Cluster, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, ok := w.clusters[name]
	return c, ok
}

// CreateCluster is sim-only arrangement: the acceptance suite conjures
// clusters with chosen properties instead of provisioning real ones.
func (w *World) CreateCluster(name string, enforcing bool) *SimCluster {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := &SimCluster{
		name:            name,
		enforcesNetPol:  enforcing,
		crds:            []cluster.CRD{},
		rawObjects:      map[string]cluster.Object{},
	}
	w.clusters[name] = c
	return c
}

// SimCluster is one simulated cluster.
type SimCluster struct {
	mu             sync.Mutex
	name           string
	enforcesNetPol bool
	crds           []cluster.CRD
	rawObjects     map[string]cluster.Object
}

func rawKey(ns, kind, name string) string { return ns + "/" + kind + "/" + name }

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
