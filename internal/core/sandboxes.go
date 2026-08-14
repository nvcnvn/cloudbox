// Sandboxes: disposable, sealed, substrate-verified environments owned by one
// developer or agent (S1–S7). A sandbox is sealed before any workload is
// admitted (N1), and only reports itself sealed after the enforcement probe
// passes (N7).
package core

import (
	"fmt"
	"time"

	"cloudbox/internal/cluster"
)

// BlockedAttempt is one recorded, attributed denied egress attempt (N4).
type BlockedAttempt struct {
	Destination string    `json:"destination"`
	At          time.Time `json:"at"`
	Workload    string    `json:"workload"`
}

// Diagnostic is a surfaced operational finding, e.g. a capacity squeeze the
// workload did not survive (S7).
type Diagnostic struct {
	Code     string `json:"code"`
	Workload string `json:"workload,omitempty"`
	Message  string `json:"message"`
}

// Sandbox is one sandbox record.
type Sandbox struct {
	Name          string `json:"name"`
	App           string `json:"app"`
	Owner         string `json:"owner"`
	Cluster       string `json:"cluster"`
	Namespace     string `json:"namespace"`
	Local         bool   `json:"local"` // user-controlled cluster (S3)
	State         string `json:"state"` // "provisioning" | "ready" | "unsealed-cluster-not-enforcing" | "destroyed"
	Sealed        bool   `json:"sealed"`
	SealVerified  bool   `json:"sealVerified"`
	ReadySeconds  float64 `json:"readySeconds"` // control-plane readiness (S2)
	AppliedDigest string `json:"appliedDigest,omitempty"`
	CapacityMode  string `json:"capacityMode,omitempty"`

	// PR binding (S6).
	PullRequest string `json:"pullRequest,omitempty"`

	CreatedAt    time.Time `json:"createdAt"`
	LastActivity time.Time `json:"lastActivity"`
	TTLSeconds   int64     `json:"ttlSeconds,omitempty"`
	// SoakStart anchors observed-healthy duration for the current digest;
	// zero while nothing healthy is running (S6).
	SoakStart time.Time `json:"soakStart,omitempty"`
	// InheritedSoak carries soak accumulated before a digest-preserving
	// re-apply (soak inheritance, S6).
	InheritedSoak time.Duration `json:"-"`

	BlockedEgress         []BlockedAttempt `json:"blockedEgress"`
	Diagnostics           []Diagnostic     `json:"diagnostics"`
	SuspendedAutoscalers  []string         `json:"suspendedAutoscalers"`
	// Datastores maps declared datastore names to their provisioned fidelity
	// level for this run (D2).
	Datastores map[string]string `json:"datastores,omitempty"`
}

// sandboxHost picks the cluster new sandboxes land on: the application's
// pinned cluster if set (CP3), else a registered sandbox cluster, else any
// cluster where setup ran. Callers hold c.mu.
func (c *Core) sandboxHost(app *Application) (cluster.Cluster, error) {
	if app != nil && app.SandboxCluster != "" {
		if cl, ok := c.driver.Cluster(app.SandboxCluster); ok {
			return cl, nil
		}
		return nil, errf(404, "application sandbox cluster %q is not known", app.SandboxCluster)
	}
	for name, roles := range c.roles {
		if roles["sandbox"] {
			if cl, ok := c.driver.Cluster(name); ok {
				return cl, nil
			}
		}
	}
	for name := range c.installed {
		if cl, ok := c.driver.Cluster(name); ok {
			return cl, nil
		}
	}
	return nil, errf(409, "no cluster available: run setup on a cluster first")
}

// CreateSandboxOptions vary one creation.
type CreateSandboxOptions struct {
	Local      bool
	TTLSeconds int64
	PR         string
}

// CreateSandbox provisions a sealed sandbox in one no-approval step (S1/S2).
func (c *Core) CreateSandbox(appName, actor string, opts CreateSandboxOptions) (*Sandbox, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.createSandboxLocked(appName, actor, opts)
}

func (c *Core) createSandboxLocked(appName, actor string, opts CreateSandboxOptions) (*Sandbox, error) {
	app, ok := c.apps[appName]
	if !ok {
		return nil, errf(404, "application %q is not known", appName)
	}

	start := c.now()
	c.sandboxSeq++
	name := fmt.Sprintf("%s-sbx-%d", appName, c.sandboxSeq)

	var host cluster.Cluster
	if opts.Local {
		// S3: one command provisions a local (user-controlled) cluster from
		// the application's substrate lockfile; same seal semantics.
		local, err := c.provisionLocalCluster(name, appName)
		if err != nil {
			return nil, err
		}
		host = local
	} else {
		var err error
		host, err = c.sandboxHost(app)
		if err != nil {
			return nil, err
		}
	}

	sb := &Sandbox{
		Name:         name,
		App:          appName,
		Owner:        actor,
		Cluster:      host.Name(),
		Namespace:    name,
		Local:        opts.Local,
		State:        "provisioning",
		CreatedAt:    start,
		LastActivity: start,
		TTLSeconds:   opts.TTLSeconds,
		PullRequest:  opts.PR,
		CapacityMode: "squeezed",
		BlockedEgress:        []BlockedAttempt{},
		Diagnostics:          []Diagnostic{},
		SuspendedAutoscalers: []string{},
	}
	c.sandboxes[name] = sb

	host.EnsureNamespace(sb.Namespace)
	if c.holdSeal {
		// Sim arrangement (N1): the seal is not yet in force; no workload can
		// be admitted until CompleteSeal runs.
		c.holdSeal = false
		return sb, nil
	}
	c.sealSandboxLocked(sb, app, host)
	sb.ReadySeconds = c.now().Sub(start).Seconds()
	return sb, nil
}

// sealSandboxLocked seals the namespace and probe-verifies enforcement.
func (c *Core) sealSandboxLocked(sb *Sandbox, app *Application, host cluster.Cluster) {
	host.SealNamespace(sb.Namespace, app.Contract.EgressAllowlist)
	if !host.ProbeEnforcement() {
		// N7: the CNI accepted the policies but does not enforce them. The
		// sandbox never reports itself sealed and produces no evidence.
		sb.State = "unsealed-cluster-not-enforcing"
		sb.Sealed = false
		sb.SealVerified = false
		return
	}
	sb.Sealed = true
	sb.SealVerified = true
	sb.State = "ready"
}

// CompleteSeal finishes a held provisioning (sim arrangement for N1).
func (c *Core) CompleteSeal(sandboxName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return errf(404, "sandbox %q is not known", sandboxName)
	}
	app := c.apps[sb.App]
	host, ok := c.driver.Cluster(sb.Cluster)
	if !ok {
		return errf(404, "cluster %q is not known", sb.Cluster)
	}
	c.sealSandboxLocked(sb, app, host)
	sb.ReadySeconds = c.now().Sub(sb.CreatedAt).Seconds()
	return nil
}

// DestroySandbox tears a sandbox down (owner-scoped, S1/S5).
func (c *Core) DestroySandbox(name, actor string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[name]
	if !ok {
		return errf(404, "sandbox %q is not known", name)
	}
	if actor != "" && sb.Owner != actor {
		return errf(403, "sandbox %q is owned by %s: only the owner may modify it (S1)", name, sb.Owner)
	}
	c.destroyLocked(sb)
	return nil
}

func (c *Core) destroyLocked(sb *Sandbox) {
	if host, ok := c.driver.Cluster(sb.Cluster); ok {
		host.DeleteNamespace(sb.Namespace)
	}
	sb.State = "destroyed"
}

// Tick applies time-based lifecycle: TTL and idle expiry (S5).
func (c *Core) Tick() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for _, sb := range c.sandboxes {
		if sb.State == "destroyed" {
			continue
		}
		if sb.TTLSeconds > 0 && now.Sub(sb.CreatedAt) >= time.Duration(sb.TTLSeconds)*time.Second {
			c.destroyLocked(sb)
			continue
		}
		app := c.apps[sb.App]
		if app != nil && app.Policies.IdleExpirySeconds > 0 &&
			now.Sub(sb.LastActivity) >= time.Duration(app.Policies.IdleExpirySeconds)*time.Second {
			c.destroyLocked(sb)
		}
	}
}

// GetSandbox returns a sandbox record.
func (c *Core) GetSandbox(name string) (*Sandbox, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[name]
	if !ok {
		return nil, errf(404, "sandbox %q is not known", name)
	}
	return sb, nil
}

// SandboxWorkloads lists what actually runs in the sandbox's namespace.
func (c *Core) SandboxWorkloads(name string) ([]cluster.Workload, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[name]
	if !ok {
		return nil, errf(404, "sandbox %q is not known", name)
	}
	host, ok := c.driver.Cluster(sb.Cluster)
	if !ok {
		return nil, nil
	}
	return host.Workloads(sb.Namespace), nil
}
