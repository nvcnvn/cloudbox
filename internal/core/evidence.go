// Evidence: the machine-gathered, control-plane-signed record of a sandbox
// run (§5, G3). This file starts as a draft record that accumulates facts as
// capabilities land; assembly, wording, and checks (G3/G6/G13) are section 5.
package core

// Evidence is the (growing) record of one sandbox's current run.
type Evidence struct {
	Sandbox      string      `json:"sandbox"`
	BundleDigest string      `json:"bundleDigest,omitempty"`
	Transforms   []Transform `json:"transforms"`
}

// evidenceFor returns the sandbox's evidence draft, creating it on first use.
// Callers hold c.mu.
func (c *Core) evidenceFor(sb *Sandbox) *Evidence {
	ev, ok := c.evidence[sb.Name]
	if !ok {
		ev = &Evidence{Sandbox: sb.Name, Transforms: []Transform{}}
		c.evidence[sb.Name] = ev
	}
	return ev
}

// GetEvidence returns the evidence draft for a sandbox.
func (c *Core) GetEvidence(sandboxName string) (*Evidence, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return nil, errf(404, "sandbox %q is not known", sandboxName)
	}
	return c.evidenceFor(sb), nil
}
