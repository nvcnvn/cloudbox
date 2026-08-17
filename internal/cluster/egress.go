// Egress attempt records observed by the product egress proxy under a real
// seal (N4, ADR 0001/0008). This is deliberately NOT part of the Cluster
// interface: cluster.go is the frozen contract (ADR 0008), and drivers that
// can observe egress expose it as an optional capability the control plane
// type-asserts for — the same pattern the sim driver's NewCluster arrangement
// hook already uses.
package cluster

import "time"

// EgressAttempt is one connection attempt the egress proxy saw.
type EgressAttempt struct {
	Destination string    `json:"destination"`
	Workload    string    `json:"workload"`
	At          time.Time `json:"at"`
	Allowed     bool      `json:"allowed"`
}

// EgressReport is one collection from a namespace's egress proxy: the
// attempts it still retains, how many its retention bound discarded, and
// which proxy process served them. The last two are what keep record loss
// visible instead of silent — a smaller count of attempts is otherwise
// indistinguishable from a quieter run (N4).
type EgressReport struct {
	Attempts []EgressAttempt `json:"attempts"`
	// Dropped is the proxy's monotonic count of attempts its retention bound
	// discarded.
	Dropped int `json:"dropped"`
	// Incarnation identifies the proxy process. A change between collections
	// means the proxy restarted, taking any uncollected attempts with it.
	Incarnation string `json:"incarnation,omitempty"`
	// Collected reports whether the proxy was read at all. A failed read is
	// not an empty record: silence must not be folded in as "no attempts",
	// nor mistaken for a restart.
	Collected bool `json:"collected"`
}

// EgressObserver is the optional driver capability: real clusters report the
// attempts their product-managed egress proxy recorded for a namespace, and
// what it could not keep.
type EgressObserver interface {
	EgressAttempts(namespace string) EgressReport
}
