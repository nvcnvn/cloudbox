// The evidence check (G13) and merge-time binding (G7): evidence is posted as
// a signed status check on the pull request by the control plane only —
// pipelines can display it, never mint it (CP4).
package core

import (
	"fmt"
	"time"
)

// MergeResult is the G7 binding decision for one merged PR.
type MergeResult struct {
	App          string    `json:"app"`
	PR           string    `json:"pr"`
	MergedDigest string    `json:"mergedDigest"`
	Status       string    `json:"status"` // "transferred" | "stale"
	Evidence     *Evidence `json:"evidence,omitempty"`
	Reason       string    `json:"reason,omitempty"`
}

// PRCheck is one status check as the SCM stores it. Anyone can post a check
// (that is how SCMs work); only the controller's carry a verifiable product
// signature.
type PRCheck struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // "pass" | "fail"
	Summary   string    `json:"summary"`
	Link      string    `json:"link,omitempty"`
	Signature string    `json:"signature,omitempty"` // empty for non-controller posters
	PostedBy  string    `json:"postedBy"`
	At        time.Time `json:"at"`
}

func prKey(app, pr string) string { return app + "/" + pr }

// bindMergeResult re-renders the merge result and decides evidence transfer
// (G7): digest match transfers evidence with its soak; mismatch marks it
// stale. Callers hold c.mu; called from the merged SCM event with the
// sandbox still live.
func (c *Core) bindMergeResult(sb *Sandbox, pr, mergedManifests string) {
	mergedDigest := BundleDigest(mergedManifests)
	result := &MergeResult{App: sb.App, PR: pr, MergedDigest: mergedDigest}
	if mergedDigest == sb.AppliedDigest {
		snapshot := *c.snapshotEvidence(sb) // soak preserved at merge time (S6)
		result.Status = "transferred"
		result.Evidence = &snapshot
	} else {
		result.Status = "stale"
		result.Reason = fmt.Sprintf(
			"merge result re-rendered to %s but the sandbox ran %s: evidence is stale until a sandbox run of the merged tree produces fresh evidence",
			mergedDigest, sb.AppliedDigest)
	}
	c.mergeResults[prKey(sb.App, pr)] = result

	// The check on the merged commit reflects the binding (G7/G13).
	if result.Status == "stale" {
		c.postCheckLocked(sb.App, pr, PRCheck{
			Name: "cloudbox/evidence", Status: "fail",
			Summary:  result.Reason,
			PostedBy: "cloudbox-controller",
		})
	}
}

// MergeResultFor reads the binding decision.
func (c *Core) MergeResultFor(app, pr string) (*MergeResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.mergeResults[prKey(app, pr)]
	if !ok {
		return nil, errf(404, "no merge result recorded for %s PR %s", app, pr)
	}
	return result, nil
}

// postCheckLocked signs (when the controller posts) and stores a status
// check. Callers hold c.mu.
func (c *Core) postCheckLocked(app, pr string, check PRCheck) PRCheck {
	check.At = c.now()
	if check.PostedBy == "cloudbox-controller" {
		check.Signature = "signed:controller:" + BundleDigest(check.Summary)[7:23]
	}
	c.prChecks[prKey(app, pr)] = append(c.prChecks[prKey(app, pr)], check)
	return check
}

// SimulatePipelineCheck models a CI pipeline hitting the SCM's status API
// directly: the SCM accepts it, but it carries no product signature (CP4).
func (c *Core) SimulatePipelineCheck(app, pr, name, summary string) PRCheck {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.postCheckLocked(app, pr, PRCheck{
		Name: name, Status: "pass", Summary: summary, PostedBy: "ci-pipeline",
	})
}

// ChecksFor lists a PR's status checks as the SCM holds them.
func (c *Core) ChecksFor(app, pr string) []PRCheck {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]PRCheck{}, c.prChecks[prKey(app, pr)]...)
}

// PostEvidenceCheck computes and posts the signed evidence check for a PR
// (G13). Validity requires: a control-plane-managed sandbox, the seal held
// with zero violations, a substrate match, fidelity at or above the policy
// minimum, and witnessed activity at or above the policy minimum.
func (c *Core) PostEvidenceCheck(sandboxName, pr string) (*PRCheck, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.requireManagedLocked(sandboxName, "post an evidence check"); err != nil {
		return nil, err
	}
	sb := c.sandboxes[sandboxName]
	app := c.apps[sb.App]
	if sb.State == "unsealed-cluster-not-enforcing" {
		return nil, errf(409, "sandbox %q produces no evidence (N7)", sandboxName)
	}
	ev := c.snapshotEvidence(sb)

	var failures []string
	if ev.SealStatus != "sealed" {
		failures = append(failures, "seal not held")
	}
	if ev.EgressViolations > 0 {
		failures = append(failures, fmt.Sprintf("seal violations: %d blocked egress attempt(s) recorded", ev.EgressViolations))
	}
	if !ev.SubstrateMatch {
		failures = append(failures, "substrate does not match production")
	}
	if min := c.applicableMinFidelity(app, ev.BundleDigest); min != "" && fidelityBelow(ev.Fidelity, min) {
		failures = append(failures, fmt.Sprintf("fidelity below the policy minimum %q", min))
	}
	if app != nil && ev.Witnessed.Events < app.Policies.MinWitnessedEvents {
		failures = append(failures, fmt.Sprintf(
			"witnessed activity %d below the policy minimum %d", ev.Witnessed.Events, app.Policies.MinWitnessedEvents))
	}

	check := PRCheck{
		Name:     "cloudbox/evidence",
		Status:   "pass",
		Summary:  ev.Summary,
		Link:     fmt.Sprintf("/v1/sandboxes/%s/evidence", sandboxName),
		PostedBy: "cloudbox-controller",
	}
	if len(failures) > 0 {
		check.Status = "fail"
		check.Summary = fmt.Sprintf("evidence check failed: %s. %s", joinAnd(failures), ev.Summary)
	}
	posted := c.postCheckLocked(sb.App, pr, check)
	c.recordAudit("cloudbox-controller", "evidence-check-posted", sandboxName,
		fmt.Sprintf("pr %s: %s", pr, posted.Status))
	return &posted, nil
}

func joinAnd(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += "; "
		}
		out += item
	}
	return out
}
