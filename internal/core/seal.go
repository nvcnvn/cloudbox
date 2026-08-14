// The seal (N1–N8, ADR 0001): default-deny with an explicit FQDN allowlist,
// recorded violations, admin-owned allowlist changes, and honestly scoped
// containment claims.
package core

import "cloudbox/internal/cluster"

// AttemptEgress evaluates one connection attempt from a workload inside a
// sandbox and records a blocked attempt with destination, timestamp, and
// attribution (N4).
func (c *Core) AttemptEgress(sandboxName, workload, destination string) (cluster.EgressResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return cluster.EgressResult{}, errf(404, "sandbox %q is not known", sandboxName)
	}
	host, ok := c.driver.Cluster(sb.Cluster)
	if !ok {
		return cluster.EgressResult{}, errf(404, "cluster %q is not known", sb.Cluster)
	}
	result := host.AttemptEgress(sb.Namespace, destination)
	if !result.Allowed {
		sb.BlockedEgress = append(sb.BlockedEgress, BlockedAttempt{
			Destination: destination,
			At:          c.now(),
			Workload:    workload,
		})
	}
	return result, nil
}

// RequestAllowlistChange is what a sandbox owner gets instead of a per-sandbox
// exception: the seal is never weakened per sandbox (N5); allowlist changes
// are application-policy changes owned by admins and audited.
func (c *Core) RequestAllowlistChange(sandboxName, actor, fqdn string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return errf(404, "sandbox %q is not known", sandboxName)
	}
	_ = sb
	return errf(403,
		"the seal is never weakened per sandbox (N5): adding %q to the allowlist is an application-policy change — submit it for admin review as an audited change to the application's boundary contract",
		fqdn)
}

// UpdateAllowlist is the admin path: an audited application-policy change.
func (c *Core) UpdateAllowlist(appName, admin string, allowlist []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[appName]
	if !ok {
		return errf(404, "application %q is not known", appName)
	}
	app.Contract.EgressAllowlist = append([]string{}, allowlist...)
	c.recordAudit(admin, "allowlist-change", appName, "egress allowlist updated")
	// Re-seal live sandboxes with the new application policy.
	for _, sb := range c.sandboxes {
		if sb.App == appName && sb.State == "ready" {
			if host, ok := c.driver.Cluster(sb.Cluster); ok {
				host.SealNamespace(sb.Namespace, allowlist)
			}
		}
	}
	return nil
}

// ContainmentStatement is the published containment claim (N8): the
// cooperative guarantee and the adversarial guarantees separately, residual
// channels named, never "unbypassable".
type ContainmentStatement struct {
	Cooperative      string   `json:"cooperative"`
	BlockedChannels  []string `json:"blockedChannels"`
	ResidualChannels []string `json:"residualChannels"`
	RoadmapV1x       []string `json:"roadmapV1x"`
}

// Containment returns the product's published containment statement (§2.5).
func (c *Core) Containment() ContainmentStatement {
	return ContainmentStatement{
		Cooperative: "closed-world reproducibility: any egress not on the allowlist is denied and recorded; " +
			"a workload that ran correctly sealed exercised zero undeclared external dependencies on the paths it ran",
		BlockedChannels: []string{
			"all direct egress, including IP-literal connections (default-deny NetworkPolicy admits only cluster DNS and the egress proxy)",
			"any FQDN not on the allowlist",
			"all ingress",
			"all writes to production (the sandbox holds no production credentials at any adoption level)",
		},
		ResidualChannels: []string{
			"low-bandwidth exfiltration through DNS queries to cluster DNS (tunneling)",
			"exfiltration through allowlisted endpoints (the seal proves where traffic went, not what it carried)",
		},
		RoadmapV1x: []string{
			"DNS query logging with per-sandbox rate limits and anomaly detection",
			"per-FQDN egress volume metering surfaced in evidence",
		},
	}
}
