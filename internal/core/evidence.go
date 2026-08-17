// Evidence: the machine-gathered, control-plane-signed record of a sandbox
// run (§5, G3), with honest wording (G6): it states what ran sealed at which
// fidelity, capacity, and duration with witnessed activity — never "verified
// working".
package core

import (
	"fmt"
	"sort"
	"strings"
)

// WitnessedActivity is what actually executed under the seal, attributed by
// the control plane (X4): idle boot is distinguished from exercised paths.
type WitnessedActivity struct {
	Tests  []TestRun `json:"tests"`
	Events int       `json:"events"`
}

// DependencyStatus reports how a declared dependency was satisfied (S8/C1).
type DependencyStatus struct {
	App    string `json:"app"`
	Status string `json:"status"` // "stubbed" (v1) | "linked" (v1.x)
}

// Evidence is the (growing) record of one sandbox's current run.
type Evidence struct {
	Sandbox      string      `json:"sandbox"`
	Source       string      `json:"source"` // "managed" | "local" (S3/CP4 trust boundary)
	BundleDigest string      `json:"bundleDigest,omitempty"`
	Transforms   []Transform `json:"transforms"`

	Diff                 []DiffLine         `json:"diff"`
	SealStatus           string             `json:"sealStatus"` // "sealed" | "not-sealed"
	EgressViolations     int                `json:"egressViolations"`
	// EgressRecordIncomplete says EgressViolations is a floor rather than a
	// total: records were lost, so the count cannot be presented as complete.
	// The alternative — a quietly diminished number in a signed artifact — is
	// worse than no number, because it is trusted (ADR 0004, N4).
	EgressRecordIncomplete bool   `json:"egressRecordIncomplete"`
	EgressRecordGap        string `json:"egressRecordGap,omitempty"`
	SubstrateDigest      string             `json:"substrateDigest,omitempty"`
	SubstrateMatch       bool               `json:"substrateMatch"`
	Fidelity             map[string]string  `json:"fidelity"` // datastore → level (D2)
	CapacityMode         string             `json:"capacityMode,omitempty"`
	AutoscalersSuspended []string           `json:"autoscalersSuspended,omitempty"`
	Readiness            string             `json:"readiness"`
	Witnessed            WitnessedActivity  `json:"witnessed"`
	Dependencies         []DependencyStatus `json:"dependencies"`

	ObservedHealthySeconds float64 `json:"observedHealthySeconds"`

	// MigrationFailures surfaces failed migration replays (D4).
	MigrationFailures []string `json:"migrationFailures,omitempty"`

	Summary   string `json:"summary"`   // G6 wording, rendered in one place
	Signature string `json:"signature"` // minted by the control plane only (CP4)

	Valid          bool     `json:"valid"`
	InvalidReasons []string `json:"invalidReasons,omitempty"`

	// substrateOverridden survives snapshot recomputation: an admin accepted
	// the mismatch and the audit log holds the override (P2).
	substrateOverridden bool
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
		ev = &Evidence{
			Sandbox: sb.Name, Source: source,
			Transforms: []Transform{}, Fidelity: map[string]string{},
			Witnessed: WitnessedActivity{Tests: []TestRun{}},
		}
		c.evidence[sb.Name] = ev
	}
	return ev
}

// snapshotEvidence refreshes computed facts, validity, wording, and signature
// before a read. Callers hold c.mu.
func (c *Core) snapshotEvidence(sb *Sandbox) *Evidence {
	ev := c.evidenceFor(sb)
	app := c.apps[sb.App]

	if sb.Sealed && sb.SealVerified {
		ev.SealStatus = "sealed"
	} else {
		ev.SealStatus = "not-sealed"
	}
	ev.EgressViolations = len(sb.BlockedEgress)
	ev.EgressRecordIncomplete = sb.EgressRecordIncomplete
	ev.EgressRecordGap = egressRecordGap(sb)
	ev.ObservedHealthySeconds = c.observedHealthy(sb).Seconds()
	ev.SubstrateDigest = c.sandboxSubstrateDigest(sb)

	// Per-datastore fidelity (D2): defaults to fixtures for anything the run
	// never provisioned beyond.
	ev.Fidelity = map[string]string{}
	for name, level := range sb.Datastores {
		ev.Fidelity[name] = level
	}

	// Declared dependency status (S8 v1: stubs).
	ev.Dependencies = nil
	if app != nil {
		for _, dep := range app.Contract.Dependencies {
			status := "stubbed"
			ev.Dependencies = append(ev.Dependencies, DependencyStatus{App: dep.App, Status: status})
		}
	}

	// Readiness: exercised vs idle is wording (G6); readiness is fact.
	ready, total := 0, 0
	if host, ok := c.driver.Cluster(sb.Cluster); ok {
		for _, w := range host.Workloads(sb.Namespace) {
			total++
			if w.Ready {
				ready++
			}
		}
	}
	ev.Readiness = fmt.Sprintf("%d/%d workloads ready", ready, total)

	if ev.BundleDigest != "" {
		if diff, err := c.diffLocked(sb.App, ev.BundleDigest); err == nil {
			ev.Diff = diff
		}
	}

	ev.Valid = true
	ev.InvalidReasons = nil
	if ev.SealStatus != "sealed" {
		ev.Valid = false
		ev.InvalidReasons = append(ev.InvalidReasons, "seal not verified")
	}
	ev.SubstrateMatch = true
	if prod, ok := c.productionCluster(); ok {
		prodDigest := c.lockfileFor(sb.App, prod).Digest
		ev.SubstrateMatch = ev.SubstrateDigest == prodDigest
		if !ev.SubstrateMatch && !ev.substrateOverridden {
			// P2: evidence is invalid on substrate digest mismatch.
			ev.Valid = false
			ev.InvalidReasons = append(ev.InvalidReasons,
				"substrate mismatch: sandbox substrate digest does not match production's")
		}
	}
	if min := c.applicableMinFidelity(app, ev.BundleDigest); min != "" && fidelityBelow(ev.Fidelity, min) {
		// D3: evidence below the applicable minimum is invalid.
		ev.Valid = false
		ev.InvalidReasons = append(ev.InvalidReasons,
			fmt.Sprintf("fidelity below the applicable minimum %q", min))
	}
	// D5: profile drift stales evidence at its declared fidelity level.
	for ds, pinned := range sb.ProfileDigests {
		if current, ok := c.profiles[dsKey(sb.App, ds)]; ok && current.Digest != pinned {
			ev.Valid = false
			ev.InvalidReasons = append(ev.InvalidReasons, fmt.Sprintf(
				"data profile drift: datastore %q moved from the profile this sandbox was provisioned from; evidence is no longer valid at declared level %q",
				ds, sb.Datastores[ds]))
		}
	}

	ev.Summary = renderSummary(ev)
	ev.Signature = signEvidence(ev)
	return ev
}

// applicableMinFidelity resolves D3's policy: the base minimum, raised by the
// conditional rule when the bundle contains a migration. Callers hold c.mu.
func (c *Core) applicableMinFidelity(app *Application, bundleDigest string) string {
	if app == nil {
		return ""
	}
	min := app.Policies.MinFidelity
	if cond := app.Policies.MinFidelityForMigrations; cond != "" && c.bundleHasMigration(bundleDigest) {
		if fidelityRank[cond] > fidelityRank[min] {
			min = cond
		}
	}
	return min
}

// fidelityRank orders the five levels (D2).
var fidelityRank = map[string]int{
	"fixtures": 0, "schema-replay": 1, "profile-synthetic": 2,
	"masked-snapshot": 3, "live-clone": 4,
}

func fidelityBelow(perDatastore map[string]string, min string) bool {
	if len(perDatastore) == 0 {
		return fidelityRank[min] > 0 // an undeclared run counts as fixtures
	}
	for _, level := range perDatastore {
		if fidelityRank[level] < fidelityRank[min] {
			return true
		}
	}
	return false
}

// renderSummary is the ONE place G6 wording is produced: scoped claims only.
func renderSummary(ev *Evidence) string {
	if ev.SealStatus != "sealed" {
		return "no claim: the seal was never verified for this run"
	}

	fidelity := "fixtures"
	if len(ev.Fidelity) > 0 {
		var levels []string
		for name, level := range ev.Fidelity {
			levels = append(levels, name+"="+level)
		}
		sort.Strings(levels)
		fidelity = strings.Join(levels, ",")
	}

	activity := fmt.Sprintf("%d test/traffic events witnessed", ev.Witnessed.Events)
	if ev.Witnessed.Events == 0 {
		activity = "0 test/traffic events witnessed — idle boot only, no code path exercised"
	}

	deps := "none declared"
	if len(ev.Dependencies) > 0 {
		var parts []string
		for _, d := range ev.Dependencies {
			parts = append(parts, d.App+": "+d.Status)
		}
		deps = strings.Join(parts, ", ")
	}

	violations := "zero undeclared dependency attempts"
	if ev.EgressViolations > 0 {
		violations = fmt.Sprintf("%d undeclared dependency attempts recorded", ev.EgressViolations)
	}
	if ev.EgressRecordIncomplete {
		// The count is a floor, and the wording says so rather than letting a
		// diminished number read as a complete one (G6).
		if ev.EgressViolations > 0 {
			violations = fmt.Sprintf(
				"at least %d undeclared dependency attempts recorded — an INCOMPLETE egress record (%s)",
				ev.EgressViolations, ev.EgressRecordGap)
		} else {
			violations = fmt.Sprintf(
				"an INCOMPLETE egress record: no undeclared dependency attempt survived collection, so zero cannot be claimed (%s)",
				ev.EgressRecordGap)
		}
	}
	substrate := "a substrate matching production"
	if !ev.SubstrateMatch {
		substrate = "a substrate NOT matching production"
	}

	return fmt.Sprintf(
		"ran sealed with %s on %s, at data fidelity %s, capacity mode %s, healthy for %.0fs, with %s, dependencies [%s]. "+
			"Claims cover exercised code paths only; no production load is implied; identity authorization and secret values are declared-not-verified.",
		violations, substrate, fidelity, ev.CapacityMode, ev.ObservedHealthySeconds, activity, deps)
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
	return c.requireManagedLocked(sandboxName, action)
}

func (c *Core) requireManagedLocked(sandboxName, action string) error {
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
	ev.substrateOverridden = true
	ev.Valid = true
	ev.InvalidReasons = nil
	c.recordAudit(admin, "substrate-mismatch-override", sandboxName, reason)
	return nil
}

