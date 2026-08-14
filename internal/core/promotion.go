// Promotion (G1, G4, G5, G8, G9, G11, G12): the approval-gated write path to
// production. Write-back is the recommended mode — the team's GitOps
// controller stays the thing that touches production; direct apply is the L4
// strict alternative. Every transition is synchronously audited.
package core

import (
	"fmt"
	"time"
)

// Promotion is one approval-gated request to apply a bundle to production.
type Promotion struct {
	ID       string   `json:"id"`
	App      string   `json:"app"`
	Sandbox  string   `json:"sandbox,omitempty"`
	Digest   string   `json:"digest"`
	Mode     string   `json:"mode"`  // "write-back" (L3) | "direct" (L4)
	State    string   `json:"state"` // pending|approved|committed|applied|failed|rejected
	OpenedBy string   `json:"openedBy"`
	Approvals []string `json:"approvals"`
	Evidence  *Evidence `json:"evidence,omitempty"`
	// History carries production history when this is a rollback (G11).
	History    *HistoryEntry `json:"history,omitempty"`
	Divergence string        `json:"divergence,omitempty"`
}

// HistoryEntry is one previously applied production bundle (G11).
type HistoryEntry struct {
	Digest              string    `json:"digest"`
	Evidence            *Evidence `json:"evidence,omitempty"`
	AppliedAt           time.Time `json:"appliedAt"`
	ObservedHealthyLive float64   `json:"observedHealthyLiveSeconds"`
}

// PromotedState tracks what an approved promotion last put live (G12).
type PromotedState struct {
	Digest        string `json:"digest"`
	EvidenceValid bool   `json:"evidenceValid"`
	Divergence    string `json:"divergence,omitempty"`
}

// auditOrFail writes a synchronous audit record; if the sink is unavailable
// the transition MUST NOT proceed (G5). Callers hold c.mu.
func (c *Core) auditOrFail(actor, action, subject, detail string) error {
	if !c.auditAvailable {
		return errf(503,
			"audit sink unavailable: the %s transition does not proceed until the audit record can be written (G5)", action)
	}
	c.recordAudit(actor, action, subject, detail)
	return nil
}

func (c *Core) promotionMode(app *Application) string {
	if app.Level == "L4" {
		return "direct"
	}
	return "write-back"
}

// openPromotionLocked creates a pending promotion request. Callers hold c.mu.
func (c *Core) openPromotionLocked(app *Application, sandbox, digest, openedBy string, evidence *Evidence, history *HistoryEntry) (*Promotion, error) {
	c.promotionSeq++
	p := &Promotion{
		ID: fmt.Sprintf("promo-%d", c.promotionSeq), App: app.Name,
		Sandbox: sandbox, Digest: digest, Mode: c.promotionMode(app),
		State: "pending", OpenedBy: openedBy, Approvals: []string{},
		Evidence: evidence, History: history,
	}
	if err := c.auditOrFail(openedBy, "promotion-created", p.ID, "digest "+digest); err != nil {
		return nil, err
	}
	c.promotions[p.ID] = p
	return p, nil
}

// OpenPromotion opens a promotion from a sandbox run (L3+).
func (c *Core) OpenPromotion(sandboxName, actor string) (*Promotion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.requireManagedLocked(sandboxName, "open a promotion"); err != nil {
		return nil, err
	}
	sb := c.sandboxes[sandboxName]
	app := c.apps[sb.App]
	ev := c.snapshotEvidence(sb)
	if !ev.Valid {
		return nil, errf(409, "promotion blocked: evidence is not valid (%v)", ev.InvalidReasons)
	}
	snapshot := *ev
	return c.openPromotionLocked(app, sandboxName, sb.AppliedDigest, actor, &snapshot, nil)
}

// Approve records one approval (G4): approvers are declared per application,
// and self-approval is rejected server-side. Enough approvals move the
// promotion to approved.
func (c *Core) Approve(promotionID, actor string) (*Promotion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.promotions[promotionID]
	if !ok {
		return nil, errf(404, "promotion %q is not known", promotionID)
	}
	app := c.apps[p.App]
	if p.OpenedBy == actor {
		return nil, errf(403, "self-approval rejected: %s opened this promotion and cannot approve it (G4)", actor)
	}
	allowed := false
	for _, approver := range app.Approvers {
		if approver == actor {
			allowed = true
		}
	}
	if !allowed {
		return nil, errf(403, "%s is not in the application's declared approver roles (G4)", actor)
	}
	for _, existing := range p.Approvals {
		if existing == actor {
			return nil, errf(409, "%s has already approved", actor)
		}
	}
	if err := c.auditOrFail(actor, "promotion-approval-recorded", p.ID, ""); err != nil {
		return nil, err
	}
	p.Approvals = append(p.Approvals, actor)

	required := app.Policies.RequiredApprovals
	if required == 0 {
		required = 1
	}
	if len(p.Approvals) >= required && p.State == "pending" {
		if err := c.auditOrFail(actor, "promotion-approved", p.ID, ""); err != nil {
			return nil, err
		}
		p.State = "approved"
	}
	return p, nil
}

// ExecuteApply runs the approved promotion's write path. Write-back commits
// the rendered bundle to the declared GitOps path and waits for the team's
// controller; direct applies the recorded bundle bytes itself (G5/G9).
func (c *Core) ExecuteApply(promotionID, actor string) (*Promotion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.promotions[promotionID]
	if !ok {
		return nil, errf(404, "promotion %q is not known", promotionID)
	}
	if p.State != "approved" {
		return nil, errf(409, "promotion %q is %s, not approved", promotionID, p.State)
	}
	bundle, ok := c.bundles[p.Digest]
	if !ok {
		return nil, errf(404, "no bundle recorded for digest %s", p.Digest)
	}

	switch p.Mode {
	case "write-back":
		app := c.apps[p.App]
		path := app.GitOpsPath
		if path == "" {
			path = "gitops/apps/" + p.App
		}
		if err := c.auditOrFail(actor, "gitops-commit", p.ID,
			fmt.Sprintf("committed %s to %s", p.Digest, path)); err != nil {
			return nil, err
		}
		c.gitops[p.App] = &gitopsCommit{Path: path, Digest: p.Digest, Manifests: bundle.ManifestYAML}
		p.State = "committed" // completion waits for the team's GitOps apply + verification
		return p, nil

	case "direct":
		if err := c.auditOrFail(actor, "promotion-applied", p.ID, "direct apply of recorded bundle bytes"); err != nil {
			return nil, err
		}
		c.setProductionObjectsLocked(p.App, bundle.ManifestYAML)
		return c.verifyPromotionLocked(p)
	}
	return nil, errf(500, "unknown promotion mode %q", p.Mode)
}

type gitopsCommit struct {
	Path      string
	Digest    string
	Manifests string
}

// GitOpsSync models the team's Argo/Flux applying the committed bundle; the
// controller then verifies the applied result against the digest (G9).
func (c *Core) GitOpsSync(appName string) (*Promotion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	commit, ok := c.gitops[appName]
	if !ok {
		return nil, errf(404, "nothing committed to the GitOps path for %q", appName)
	}
	manifests := commit.Manifests
	if c.failNextSync[appName] {
		// A partial apply: the live state diverges from the commit (G11).
		delete(c.failNextSync, appName)
		manifests = commit.Manifests + "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: partial-apply-leftover\n"
	}
	c.setProductionObjectsLocked(appName, manifests)

	var p *Promotion
	for _, candidate := range c.promotions {
		if candidate.App == appName && candidate.State == "committed" {
			p = candidate
		}
	}
	if p == nil {
		return nil, errf(404, "no committed promotion awaiting verification for %q", appName)
	}
	return c.verifyPromotionLocked(p)
}

// verifyPromotionLocked completes a promotion only when live state matches
// the bundle digest; anything else is failed with divergence recorded and the
// rollback path available (G9/G11). Callers hold c.mu.
func (c *Core) verifyPromotionLocked(p *Promotion) (*Promotion, error) {
	live := c.production[p.App]
	if live == nil || live.Digest != p.Digest {
		liveDigest := "(nothing live)"
		if live != nil {
			liveDigest = live.Digest
		}
		p.State = "failed"
		p.Divergence = fmt.Sprintf("live state is %s, expected %s", liveDigest, p.Digest)
		if err := c.auditOrFail("cloudbox-controller", "promotion-failed", p.ID, p.Divergence); err != nil {
			return nil, err
		}
		return p, nil
	}
	if p.State != "applied" {
		if p.Mode == "write-back" {
			if err := c.auditOrFail("cloudbox-controller", "promotion-applied", p.ID,
				"live state verified against digest"); err != nil {
				return nil, err
			}
		}
		p.State = "applied"
		c.finishApplyLocked(p)
	}
	return p, nil
}

// finishApplyLocked updates promoted state and production history. Callers
// hold c.mu.
func (c *Core) finishApplyLocked(p *Promotion) {
	now := c.now()
	if prev := c.lastHistory(p.App); prev != nil {
		prev.ObservedHealthyLive = now.Sub(prev.AppliedAt).Seconds()
	}
	c.history[p.App] = append(c.history[p.App], &HistoryEntry{
		Digest: p.Digest, Evidence: p.Evidence, AppliedAt: now,
	})
	c.promoted[p.App] = &PromotedState{Digest: p.Digest, EvidenceValid: true}
}

func (c *Core) lastHistory(app string) *HistoryEntry {
	entries := c.history[app]
	if len(entries) == 0 {
		return nil
	}
	return entries[len(entries)-1]
}

// OpenRollback opens a promotion for a previously applied digest in one
// command, carrying the original evidence plus its production history (G11).
func (c *Core) OpenRollback(appName, digest, actor string) (*Promotion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[appName]
	if !ok {
		return nil, errf(404, "application %q is not known", appName)
	}
	for _, entry := range c.history[appName] {
		if entry.Digest == digest {
			now := c.now()
			snapshot := *entry
			if snapshot.ObservedHealthyLive == 0 {
				snapshot.ObservedHealthyLive = now.Sub(entry.AppliedAt).Seconds()
			}
			return c.openPromotionLocked(app, "", digest, actor, entry.Evidence, &snapshot)
		}
	}
	return nil, errf(404, "digest %s was never applied to production for %q", digest, appName)
}

// ListPromotions lists promotions, optionally filtered by application.
func (c *Core) ListPromotions(appName string) []*Promotion {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []*Promotion
	for _, p := range c.promotions {
		if appName == "" || p.App == appName {
			out = append(out, p)
		}
	}
	return out
}

// GetPromotion reads one promotion.
func (c *Core) GetPromotion(id string) (*Promotion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.promotions[id]
	if !ok {
		return nil, errf(404, "promotion %q is not known", id)
	}
	return p, nil
}

// SetAuditSink toggles the audit sink (sim arrangement for G5).
func (c *Core) SetAuditSink(available bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auditAvailable = available
}

// FailNextSync arranges a partial GitOps apply (sim arrangement for G11).
func (c *Core) FailNextSync(appName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failNextSync[appName] = true
}
