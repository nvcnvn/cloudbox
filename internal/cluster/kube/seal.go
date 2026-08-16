// The seal on a real cluster (N1–N8, ADR 0001, ADR 0008): standard
// NetworkPolicy v1 admitting only cluster DNS and the egress proxy, verified
// by a live enforcement probe.
//
// Sealing and probing are implemented by tasks 3.6–3.8 of
// add-kube-driver-conformance. Until they land, this driver fails honestly:
// the probe reports unenforced, so no sandbox on a real cluster is reported
// sealed — the exact posture ADR 0004 demands when enforcement is unproven.
package kube

import (
	"log"

	"cloudbox/internal/cluster"
)

// SealNamespace installs the default-deny NetworkPolicy floor. Not yet
// implemented for real clusters (task 3.6); the enforcement probe below
// guarantees the gap cannot produce a falsely sealed sandbox.
func (c *Cluster) SealNamespace(name string, allowlist []string) {
	log.Printf("kube driver: SealNamespace(%q) not yet implemented on %s — the enforcement probe will fail", name, c.name)
}

// ProbeEnforcement verifies a denied connection is actually denied (N7).
// Until the real probe lands (task 3.8), enforcement is unproven, and
// unproven enforcement MUST read as failure, never as a pass.
func (c *Cluster) ProbeEnforcement() bool {
	return false
}

// AttemptEgress evaluates one connection attempt under whatever the cluster
// actually enforces. Implemented by task 3.7; unproven enforcement reports
// the honest silent-failure mode: nothing filters the connection.
func (c *Cluster) AttemptEgress(namespace, destination string) cluster.EgressResult {
	return cluster.EgressResult{Allowed: true, Via: "unfiltered"}
}
