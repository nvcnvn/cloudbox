// SCM integration (S6, G7, G13, CP4): the controller receives webhook events
// and drives PR-bound sandboxes. One branch = one sandbox.
package core

import "fmt"

// SCMEvent is one webhook delivery from the SCM integration.
type SCMEvent struct {
	Type      string `json:"type"` // "opened" | "push" | "closed" | "merged"
	App       string `json:"app"`
	PR        string `json:"pr"`
	Author    string `json:"author"`
	Manifests string `json:"manifests"` // rendered bundle for this push
}

// prSandboxName maps one branch/PR to its one sandbox (S6). Callers hold c.mu.
func (c *Core) prSandbox(app, pr string) *Sandbox {
	for _, sb := range c.sandboxes {
		if sb.App == app && sb.PullRequest == pr && sb.State != "destroyed" {
			return sb
		}
	}
	return nil
}

// HandleSCMEvent drives the PR-bound sandbox lifecycle (S6): created on open,
// re-rendered and re-applied on every push, expired on close or merge.
func (c *Core) HandleSCMEvent(ev SCMEvent) (*Sandbox, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[ev.App]
	if !ok {
		return nil, errf(404, "application %q is not known", ev.App)
	}
	if !app.SCMIntegration {
		return nil, errf(409, "application %q has no SCM integration enabled", ev.App)
	}

	switch ev.Type {
	case "opened":
		if existing := c.prSandbox(ev.App, ev.PR); existing != nil {
			return existing, nil
		}
		return c.createSandboxLocked(ev.App, ev.Author, CreateSandboxOptions{PR: ev.PR})

	case "push":
		sb := c.prSandbox(ev.App, ev.PR)
		if sb == nil {
			var err error
			sb, err = c.createSandboxLocked(ev.App, ev.Author, CreateSandboxOptions{PR: ev.PR})
			if err != nil {
				return nil, err
			}
		}
		newDigest := BundleDigest(ev.Manifests)
		if sb.AppliedDigest == newDigest {
			// Soak inheritance (S6): an identical rendered digest preserves
			// accumulated soak — nothing is re-applied, nothing resets.
			return sb, nil
		}
		c.mu.Unlock()
		_, err := c.Apply(ev.App, sb.Name, sb.Owner, ev.Manifests, ApplyOptions{})
		c.mu.Lock()
		if err != nil {
			return nil, fmt.Errorf("re-apply on push failed: %w", err)
		}
		return sb, nil

	case "closed", "merged":
		sb := c.prSandbox(ev.App, ev.PR)
		if sb == nil {
			return nil, errf(404, "no sandbox bound to PR %s", ev.PR)
		}
		if ev.Type == "merged" && ev.Manifests != "" {
			// G7: re-render the merge result and bind evidence by digest
			// before the sandbox goes away.
			c.bindMergeResult(sb, ev.PR, ev.Manifests)
		}
		// TTL fires on PR close or merge (S6).
		c.destroyLocked(sb)
		return sb, nil
	}
	return nil, errf(422, "unknown SCM event type %q", ev.Type)
}
