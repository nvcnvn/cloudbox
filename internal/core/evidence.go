// Evidence: the machine-gathered, control-plane-signed record of a sandbox
// run (§5, G3). Facts accumulate here as the run proceeds; assembly, honest
// wording, and the signed check (G3/G6/G13) are the evidence capability.
package core

// Evidence is the (growing) record of one sandbox's current run.
type Evidence struct {
	Sandbox      string      `json:"sandbox"`
	Source       string      `json:"source"` // "managed" | "local" (S3/CP4 trust boundary)
	BundleDigest string      `json:"bundleDigest,omitempty"`
	Transforms   []Transform `json:"transforms"`

	SealStatus           string   `json:"sealStatus"` // "sealed" | "not-sealed"
	EgressViolations     int      `json:"egressViolations"`
	SubstrateDigest      string   `json:"substrateDigest,omitempty"`
	CapacityMode         string   `json:"capacityMode,omitempty"`
	AutoscalersSuspended []string `json:"autoscalersSuspended,omitempty"`

	ObservedHealthySeconds float64 `json:"observedHealthySeconds"`

	Valid          bool     `json:"valid"`
	InvalidReasons []string `json:"invalidReasons,omitempty"`
}

// evidenceFor returns the sandbox's evidence draft, creating it on first use.
// Callers hold c.mu.
func (c *Core) evidenceFor(sb *Sandbox) *Evidence {
	ev, ok := c.evidence[sb.Name]
	if !ok {
		source := "managed"
		if sb.Local {
			source = "local"
		}
		ev = &Evidence{Sandbox: sb.Name, Source: source, Transforms: []Transform{}}
		c.evidence[sb.Name] = ev
	}
	return ev
}

// snapshotEvidence refreshes computed facts and validity before a read.
// Callers hold c.mu.
func (c *Core) snapshotEvidence(sb *Sandbox) *Evidence {
	ev := c.evidenceFor(sb)
	if sb.Sealed && sb.SealVerified {
		ev.SealStatus = "sealed"
	} else {
		ev.SealStatus = "not-sealed"
	}
	ev.EgressViolations = len(sb.BlockedEgress)
	ev.ObservedHealthySeconds = c.observedHealthy(sb).Seconds()
	ev.SubstrateDigest = c.sandboxSubstrateDigest(sb)

	ev.Valid = true
	ev.InvalidReasons = nil
	if ev.SealStatus != "sealed" {
		ev.Valid = false
		ev.InvalidReasons = append(ev.InvalidReasons, "seal not verified")
	}
	if prod, ok := c.productionCluster(); ok {
		prodDigest := c.lockfileFor(sb.App, prod).Digest
		if ev.SubstrateDigest != prodDigest {
			// P2: evidence is invalid on substrate digest mismatch.
			ev.Valid = false
			ev.InvalidReasons = append(ev.InvalidReasons,
				"substrate mismatch: sandbox substrate digest does not match production's")
		}
	}
	return ev
}

// GetEvidence returns the evidence record for a sandbox. A sandbox whose seal
// was never verified produces no evidence at all (N7).
func (c *Core) GetEvidence(sandboxName string) (*Evidence, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return nil, errf(404, "sandbox %q is not known", sandboxName)
	}
	if sb.State == "unsealed-cluster-not-enforcing" {
		return nil, errf(409,
			"sandbox %q produces no evidence: the cluster's CNI did not pass the seal enforcement probe (N7)",
			sandboxName)
	}
	return c.snapshotEvidence(sb), nil
}

// RequireManagedSandbox enforces the trust boundary: evidence from a
// user-controlled (local) sandbox is forgeable by construction, so it can
// never back a check or a promotion (S3, CP4).
func (c *Core) RequireManagedSandbox(sandboxName, action string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return errf(404, "sandbox %q is not known", sandboxName)
	}
	if sb.Local {
		return errf(403,
			"cannot %s: sandbox %q runs on a user-controlled local cluster, so its evidence is not from a control-plane-managed sandbox and is non-postable and non-promotable (S3)",
			action, sandboxName)
	}
	return nil
}

// OverrideSubstrateMismatch records an audited admin override of a substrate
// invalidation (P2).
func (c *Core) OverrideSubstrateMismatch(sandboxName, admin, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return errf(404, "sandbox %q is not known", sandboxName)
	}
	ev := c.evidenceFor(sb)
	ev.Valid = true
	ev.InvalidReasons = nil
	c.recordAudit(admin, "substrate-mismatch-override", sandboxName, reason)
	return nil
}
