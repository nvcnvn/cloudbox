// Sandboxes: disposable, sealed, substrate-verified environments owned by one
// developer or agent (S1–S7). A sandbox is sealed before any workload is
// admitted (N1), and only reports itself sealed after the enforcement probe
// passes (N7).
package core

import (
	"fmt"
	"strings"
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
	// EgressDropped counts attempts the namespace's egress proxy discarded to
	// stay inside its retention bound. Truncation is reported, never silent:
	// with a non-zero count, BlockedEgress is a floor, not a total (N4).
	EgressDropped int `json:"egressDropped"`
	// egressDroppedSeen is the proxy's own monotonic counter as of the last
	// collection, so repeated collections add each drop once. It resets with
	// the proxy's incarnation, since a fresh process counts from zero.
	egressDroppedSeen int
	// EgressProxyRestarts counts proxy restarts seen after a collection. The
	// record lives in the proxy's memory, so a restart takes whatever was not
	// yet collected with it.
	EgressProxyRestarts int `json:"egressProxyRestarts"`
	// EgressRecordIncomplete says the control plane cannot claim this
	// sandbox's egress record is complete — attempts were dropped or a proxy
	// restarted holding uncollected ones. Deliberately conservative: a restart
	// that happened to lose nothing still marks the record incomplete, because
	// a stateless proxy cannot prove the negative. Admitting the doubt is the
	// only honest option when the alternative is claiming a completeness that
	// cannot be demonstrated (N4).
	EgressRecordIncomplete bool `json:"egressRecordIncomplete"`
	// egressIncarnation is the proxy process the last collection came from.
	egressIncarnation string

	Diagnostics           []Diagnostic     `json:"diagnostics"`
	SuspendedAutoscalers  []string         `json:"suspendedAutoscalers"`
	// Datastores maps declared datastore names to their provisioned fidelity
	// level for this run (D2).
	Datastores map[string]string `json:"datastores,omitempty"`
	// ProfileDigests pins the data profile each shape-claiming datastore was
	// provisioned from, so drift can stale it (D5).
	ProfileDigests map[string]string `json:"profileDigests,omitempty"`
	// CloneEndpoints/CloneSecrets are per-sandbox thin-clone wiring through
	// the boundary contract (D7).
	CloneEndpoints map[string]string `json:"cloneEndpoints,omitempty"`
	CloneSecrets   map[string]string `json:"cloneSecrets,omitempty"`
	// AgentOwned marks sandboxes owned by AI agents; real-data levels need
	// explicit policy for them (D8).
	AgentOwned bool `json:"agentOwned,omitempty"`
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
	Agent      bool
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
		AgentOwned:   opts.Agent,
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
	c.refreshObservedDiagnosticsLocked(sb)
	c.refreshBlockedEgressLocked(sb)
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
	c.refreshObservedDiagnosticsLocked(sb)
	return host.Workloads(sb.Namespace), nil
}

// EgressCollectionInterval is how often the control plane collects the egress
// proxies' attempt records.
//
// 15s, settled by measurement on the conformance cluster rather than by taste.
// The interval buys two things and costs one. It bounds the window of loss: a
// proxy that dies takes at most one interval of uncollected attempts with it,
// and that loss is surfaced rather than absorbed (a restart marks the record
// incomplete), so the interval does not have to beat pod rescheduling — it
// only has to keep the window small. It also has to be short enough for the
// conformance run to observe collection happening without inspecting the
// sandbox, which it does by dwelling three intervals.
//
// The cost is one service-proxy read per sealed sandbox per round, measured at
// ~30ms against the enforcing Kind cluster, and the round runs serially. At
// 15s that is well under 1% duty for the fleet sizes this driver serves. The
// honest limit: a fleet large enough for a round to approach the interval
// needs collection moved off the core lock before the interval is shortened,
// because a round currently holds it for its whole duration.
const EgressCollectionInterval = 15 * time.Second

// CollectEgress folds every live sandbox's egress-proxy record into the
// control plane. Collection MUST NOT depend on someone reading the sandbox:
// the proxy holds its attempts in memory, so an uninspected sandbox whose
// proxy restarts loses whatever was never collected, and the run's evidence
// then reports fewer violations than actually happened (N4). Reads still
// refresh (refreshBlockedEgressLocked) — that path is no longer the only one.
func (c *Core) CollectEgress() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, sb := range c.sandboxes {
		if sb.State == "destroyed" || !sb.Sealed {
			continue
		}
		c.refreshBlockedEgressLocked(sb)
	}
}

// refreshBlockedEgressLocked folds the egress proxy's observed denials into
// the sandbox's blocked-egress record (N4). The sim driver records attempts
// synchronously through core.AttemptEgress; on a real cluster the product
// egress proxy observes them, and drivers expose that as the optional
// cluster.EgressObserver capability (ADR 0008 keeps the cluster contract
// frozen). Deduplicated by destination, workload and timestamp. Callers hold
// c.mu.
func (c *Core) refreshBlockedEgressLocked(sb *Sandbox) {
	if sb.State == "destroyed" {
		return
	}
	host, ok := c.driver.Cluster(sb.Cluster)
	if !ok {
		return
	}
	observer, ok := host.(cluster.EgressObserver)
	if !ok {
		return
	}
	report := observer.EgressAttempts(sb.Namespace)
	if !report.Collected {
		// The proxy could not be read. Silence is not an empty record, and it
		// is not a restart either.
		return
	}
	// A different process is answering than the one the last collection came
	// from: the proxy restarted, and anything it had not handed over is gone.
	// Its drop counter starts from zero again with it.
	if sb.egressIncarnation != "" && report.Incarnation != "" &&
		report.Incarnation != sb.egressIncarnation {
		sb.EgressProxyRestarts++
		sb.egressDroppedSeen = 0
	}
	if report.Incarnation != "" {
		sb.egressIncarnation = report.Incarnation
	}
	// What the proxy's bound discarded is added once per drop, not once per
	// collection: the counter it reports is monotonic within one incarnation.
	if report.Dropped > sb.egressDroppedSeen {
		sb.EgressDropped += report.Dropped - sb.egressDroppedSeen
	}
	sb.egressDroppedSeen = report.Dropped
	sb.EgressRecordIncomplete = sb.EgressDropped > 0 || sb.EgressProxyRestarts > 0
	for _, attempt := range report.Attempts {
		if attempt.Allowed {
			continue
		}
		recorded := false
		for _, b := range sb.BlockedEgress {
			if b.Destination == attempt.Destination && b.Workload == attempt.Workload &&
				b.At.Equal(attempt.At) {
				recorded = true
			}
		}
		if !recorded {
			sb.BlockedEgress = append(sb.BlockedEgress, BlockedAttempt{
				Destination: attempt.Destination,
				At:          attempt.At,
				Workload:    attempt.Workload,
			})
		}
	}
}

// egressRecordGap states why a sandbox's egress record cannot be claimed
// complete, in the terms the loss actually happened in. Empty when the record
// is whole. This is the one place the gap is worded, so the sandbox record and
// the run's evidence say the same thing.
func egressRecordGap(sb *Sandbox) string {
	var parts []string
	if sb.EgressProxyRestarts > 0 {
		restarts := "the egress proxy restarted"
		if sb.EgressProxyRestarts > 1 {
			restarts = fmt.Sprintf("the egress proxy restarted %d times", sb.EgressProxyRestarts)
		}
		parts = append(parts, restarts+
			" holding records that had not been collected; what it held is unrecoverable")
	}
	if sb.EgressDropped > 0 {
		parts = append(parts, fmt.Sprintf(
			"the proxy's retention bound discarded %d attempt(s)", sb.EgressDropped))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ")
}

// refreshObservedDiagnosticsLocked folds cluster-observed workload failures
// into the sandbox record. The sim driver's arranged OOM kills are recorded
// at admission; on a real cluster the kill happens after admission and only
// the cluster's own status shows it (ADR 0008), so reads reconcile the
// record with what the driver observes. Deduplicated by workload, so the
// sim's admission-time diagnostics never double up. Callers hold c.mu.
func (c *Core) refreshObservedDiagnosticsLocked(sb *Sandbox) {
	if sb.State == "destroyed" || sb.CapacityMode == "" || sb.CapacityMode == "full" {
		return
	}
	host, ok := c.driver.Cluster(sb.Cluster)
	if !ok {
		return
	}
	for _, w := range host.Workloads(sb.Namespace) {
		if !w.OOMKilled {
			continue
		}
		recorded := false
		for _, d := range sb.Diagnostics {
			if d.Code == "capacity-squeeze-incompatible" && d.Workload == w.Name {
				recorded = true
			}
		}
		if !recorded {
			sb.Diagnostics = append(sb.Diagnostics, Diagnostic{
				Code:     "capacity-squeeze-incompatible",
				Workload: w.Name,
				Message: fmt.Sprintf(
					"workload %q was OOM-killed under the %s capacity transform; its memory floor is below what the workload needs — configure capacity: full for this application",
					w.Name, sb.CapacityMode),
			})
		}
	}
}
