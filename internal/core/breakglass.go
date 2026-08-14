// Break-glass (G12): a named emergency role obtains auto-expiring direct
// write access with no approval at grant time; every action is audited, every
// resulting divergence invalidates evidence until a reconciling promotion.
package core

import "time"

const breakGlassTTL = time.Hour

// BreakGlassGrant is one issued emergency credential.
type BreakGlassGrant struct {
	App       string    `json:"app"`
	Actor     string    `json:"actor"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// GrantBreakGlass issues auto-expiring credentials to the configured
// emergency role, immediately and without approval (G12).
func (c *Core) GrantBreakGlass(appName, actor string) (*BreakGlassGrant, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[appName]
	if !ok {
		return nil, errf(404, "application %q is not known", appName)
	}
	named := false
	for _, role := range app.BreakGlassRole {
		if role == actor {
			named = true
		}
	}
	if !named {
		return nil, errf(403, "%s is not in the application's configured break-glass role", actor)
	}
	if c.breakGlass[appName] == nil {
		c.breakGlass[appName] = map[string]time.Time{}
	}
	expires := c.now().Add(breakGlassTTL)
	c.breakGlass[appName][actor] = expires
	c.recordAudit(actor, "break-glass-granted", appName, "auto-expiring credentials, no approval at grant time")
	return &BreakGlassGrant{App: appName, Actor: actor, ExpiresAt: expires}, nil
}

// hasBreakGlassLocked reports live credentials. Callers hold c.mu.
func (c *Core) hasBreakGlassLocked(appName, actor string) bool {
	expiry, ok := c.breakGlass[appName][actor]
	return ok && c.now().Before(expiry)
}

// ProductionStatus reports the promoted-vs-live relationship (G12).
func (c *Core) ProductionStatus(appName string) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.apps[appName]; !ok {
		return nil, errf(404, "application %q is not known", appName)
	}
	out := map[string]any{"promoted": c.promoted[appName]}
	if live := c.production[appName]; live != nil {
		out["liveDigest"] = live.Digest
	}
	return out, nil
}

// CredentialsReport states what the product can touch at this application's
// adoption level (G1): below L4 it holds no production write credentials.
func (c *Core) CredentialsReport(appName string) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[appName]
	if !ok {
		return nil, errf(404, "application %q is not known", appName)
	}
	return map[string]any{
		"level":                      app.Level,
		"productionWriteCredentials": app.Level == "L4",
		"gitopsRepoWrite":            app.Level == "L3" || app.Level == "L4",
		"posture":                    "production is observed and verified, never controlled, below strict mode",
	}, nil
}
