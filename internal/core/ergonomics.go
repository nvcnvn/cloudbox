// Developer ergonomics (X1/X2): sealed logs/exec/port-forward, owner-scoped
// and audited, and the explain loop that turns blocked egress into an
// allowlist change proposal. Living inside the seal must beat docker-compose.
package core

import (
	"fmt"

	"cloudbox/internal/cluster"
)

// ownedWorkload resolves a workload in a sandbox the actor owns. Callers hold
// c.mu.
func (c *Core) ownedWorkload(sandboxName, workload, actor string) (*Sandbox, *cluster.Workload, error) {
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return nil, nil, errf(404, "sandbox %q is not known", sandboxName)
	}
	if sb.Owner != actor {
		return nil, nil, errf(403, "sandbox %q is owned by %s: only the owner may modify it (S1)", sandboxName, sb.Owner)
	}
	host, ok := c.driver.Cluster(sb.Cluster)
	if !ok {
		return nil, nil, errf(404, "cluster %q is not known", sb.Cluster)
	}
	for _, w := range host.Workloads(sb.Namespace) {
		if w.Name == workload {
			found := w
			return sb, &found, nil
		}
	}
	return nil, nil, errf(404, "workload %q is not running in sandbox %q", workload, sandboxName)
}

// Logs streams a sealed workload's logs to its owner, audited (X1).
func (c *Core) Logs(sandboxName, workload, actor string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, w, err := c.ownedWorkload(sandboxName, workload, actor)
	if err != nil {
		return "", err
	}
	sb.LastActivity = c.now()
	c.recordAudit(actor, "logs", sandboxName, "workload "+workload)
	return fmt.Sprintf("streaming logs for %s in %s:\n%s ready=%t\n", workload, sandboxName, w.Name, w.Ready), nil
}

// Exec runs a command in a sealed workload, owner-scoped and audited (X1).
func (c *Core) Exec(sandboxName, workload, command, actor string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, _, err := c.ownedWorkload(sandboxName, workload, actor)
	if err != nil {
		return "", err
	}
	sb.LastActivity = c.now()
	c.recordAudit(actor, "exec", sandboxName, "workload "+workload+": "+command)
	return fmt.Sprintf("exec %q in %s/%s: ok\n", command, sandboxName, workload), nil
}

// PortForwardResult reports how ingress-for-one-developer works: through the
// control plane, never as a seal exception (X1).
type PortForwardResult struct {
	Via      string `json:"via"`
	LocalURL string `json:"localUrl"`
}

// PortForward tunnels the owner to a sealed workload through the control
// plane, audited (X1). The seal is untouched: no ingress rule is added.
func (c *Core) PortForward(sandboxName, workload string, port int, actor string) (*PortForwardResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, _, err := c.ownedWorkload(sandboxName, workload, actor)
	if err != nil {
		return nil, err
	}
	sb.LastActivity = c.now()
	c.recordAudit(actor, "port-forward", sandboxName, fmt.Sprintf("workload %s port %d", workload, port))
	return &PortForwardResult{
		Via:      "control-plane",
		LocalURL: fmt.Sprintf("http://127.0.0.1:%d", port),
	}, nil
}

// AllowlistProposal is the ready-to-submit change explain emits (X2): the
// seal teaches the allowlist, ownership stays with admins (N5).
type AllowlistProposal struct {
	App       string   `json:"app"`
	AddFQDNs  []string `json:"addFqdns"`
	SubmitTo  string   `json:"submitTo"`
}

// Explanation is the status --explain payload (X2).
type Explanation struct {
	Sandbox       string             `json:"sandbox"`
	BlockedEgress []BlockedAttempt   `json:"blockedEgress"`
	Proposal      *AllowlistProposal `json:"proposal,omitempty"`
}

// Explain renders every blocked egress attempt and emits the allowlist change
// proposal for admin review (X2).
func (c *Core) Explain(sandboxName string) (*Explanation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return nil, errf(404, "sandbox %q is not known", sandboxName)
	}
	out := &Explanation{
		Sandbox:       sandboxName,
		BlockedEgress: append([]BlockedAttempt{}, sb.BlockedEgress...),
	}
	seen := map[string]bool{}
	var fqdns []string
	for _, attempt := range sb.BlockedEgress {
		if !seen[attempt.Destination] {
			seen[attempt.Destination] = true
			fqdns = append(fqdns, attempt.Destination)
		}
	}
	if len(fqdns) > 0 {
		out.Proposal = &AllowlistProposal{
			App:      sb.App,
			AddFQDNs: fqdns,
			SubmitTo: "admin review: audited application-policy change (N5)",
		}
	}
	return out, nil
}
