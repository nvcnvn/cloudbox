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

// EgressObserver is the optional driver capability: real clusters report the
// attempts their product-managed egress proxy recorded for a namespace.
type EgressObserver interface {
	EgressAttempts(namespace string) []EgressAttempt
}
