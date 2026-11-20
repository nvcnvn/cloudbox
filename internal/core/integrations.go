package core

import "cloudbox/internal/refusal"

// Integrations records which external systems this control plane is wired to
// (ADR 0009). An operation that depends on an absent integration refuses,
// names the integration, and records nothing — it never writes to a
// simulated surface and reports success.
//
// Presence is a property of the deployment, not of the driver's name: the sim
// driver's arrangement surfaces constitute presence, so the sim suite and the
// released kube driver follow one rule.
type Integrations struct {
	SourceControl    bool `json:"sourceControl"`
	GitOps           bool `json:"gitops"`
	ProductionIngest bool `json:"productionIngest"`
}

// Integration names as refusals state them.
const (
	IntegrationSourceControl    = "source control"
	IntegrationGitOps           = "GitOps"
	IntegrationProductionIngest = "production ingest"
)

// SetIntegrations declares which integrations are present.
func (c *Core) SetIntegrations(in Integrations) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.integrations = in
}

// Integrations reports which integrations are present.
func (c *Core) Integrations() Integrations {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.integrations
}

// requireIntegration is the integration-presence predicate. Callers hold c.mu
// and call it before any state is touched, so a refusal records nothing.
func (c *Core) requireIntegration(present bool, name, operation string) error {
	if present {
		return nil
	}
	return refuse(501, refusal.IntegrationMissing,
		"cannot %s: no %s integration is configured for this control plane, so the request is refused and nothing was recorded",
		operation, name)
}
