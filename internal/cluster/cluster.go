// Package cluster defines what the control plane needs from a Kubernetes
// cluster. The sim driver implements it for acceptance (ADR 0007); the kube
// driver is the production path behind the same interface.
package cluster

// CRD identifies a custom resource definition installed on a cluster.
type CRD struct {
	Name  string `json:"name"`  // e.g. applications.cloudbox.dev
	Group string `json:"group"` // e.g. cloudbox.dev
	Kind  string `json:"kind"`  // e.g. Application
}

// Object is a Kubernetes object as the user wrote it. The control plane never
// wraps or re-schematizes user workloads (CP1), so the manifest travels whole.
type Object struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	Namespace  string         `json:"namespace,omitempty"`
	Manifest   map[string]any `json:"manifest"`
}

// FromManifest builds an Object from a parsed manifest, reading the standard
// identity fields and keeping the manifest itself untouched.
func FromManifest(manifest map[string]any) Object {
	obj := Object{Manifest: manifest}
	if v, ok := manifest["apiVersion"].(string); ok {
		obj.APIVersion = v
	}
	if v, ok := manifest["kind"].(string); ok {
		obj.Kind = v
	}
	if meta, ok := manifest["metadata"].(map[string]any); ok {
		if v, ok := meta["name"].(string); ok {
			obj.Name = v
		}
		if v, ok := meta["namespace"].(string); ok {
			obj.Namespace = v
		}
	}
	return obj
}

// Driver is a fleet of registered clusters.
type Driver interface {
	Cluster(name string) (Cluster, bool)
}

// Cluster is one cluster's control surface, growing per capability.
type Cluster interface {
	InstallCRDs(crds []CRD)
	ListCRDs() []CRD
	// ApplyRaw stores an object exactly as given — the user's own kubectl
	// path, used to verify CloudBox coexists without wrapping anything.
	ApplyRaw(obj Object)
	GetRaw(namespace, kind, name string) (Object, bool)
}
