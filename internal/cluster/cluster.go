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

// Component is one substrate element installed on a cluster: an operator
// release owning CRD groups, an admission configuration, or a named class.
type Component struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Kind     string   `json:"kind"` // "operator" | "admission" | "class"
	OwnsCRDs []string `json:"ownsCRDs,omitempty"` // API groups, e.g. cert-manager.io
	Classes  []string `json:"classes,omitempty"`  // class names this component provides
}

// Workload is a running unit inside a namespace, admitted from a bundle.
type Workload struct {
	Name      string         `json:"name"`
	Ready     bool           `json:"ready"`
	OOMKilled bool           `json:"oomKilled"`
	Manifest  map[string]any `json:"manifest"`
}

// EgressResult reports how a connection attempt fared under the seal.
type EgressResult struct {
	Allowed bool   `json:"allowed"`
	Via     string `json:"via"` // "in-sandbox" | "cluster-dns" | "egress-proxy" | "unfiltered" | "denied"
}

// Driver is a fleet of registered clusters.
type Driver interface {
	Cluster(name string) (Cluster, bool)
}

// Cluster is one cluster's control surface.
type Cluster interface {
	Name() string
	// UserControlled reports whether the cluster is outside control-plane
	// custody (a developer's local Kind cluster): its evidence is forgeable
	// by construction (S3, CP4).
	UserControlled() bool

	InstallCRDs(crds []CRD)
	ListCRDs() []CRD
	ApplyRaw(obj Object)
	GetRaw(namespace, kind, name string) (Object, bool)

	// Substrate.
	KubernetesMinor() string
	Components() []Component
	SetComponents(k8sMinor string, comps []Component)

	// Namespaces and the seal.
	EnsureNamespace(name string)
	DeleteNamespace(name string)
	// SealNamespace installs default-deny ingress/egress expressed as standard
	// NetworkPolicy whose only admitted egress is cluster DNS and the egress
	// proxy; the proxy enforces the FQDN allowlist (N2/N3, ADR 0001).
	SealNamespace(name string, allowlist []string)
	// ProbeEnforcement creates a canary workload and confirms a denied
	// connection is actually denied (N7). A CNI that ignores NetworkPolicy
	// fails this probe.
	ProbeEnforcement() bool
	// AttemptEgress evaluates one connection attempt from inside a namespace
	// under whatever the cluster actually enforces.
	AttemptEgress(namespace, destination string) EgressResult

	// Workloads.
	AddWorkload(namespace string, w Workload)
	Workloads(namespace string) []Workload
}
