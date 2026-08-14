// Sandboxes: disposable, sealed, substrate-verified environments owned by one
// developer or agent (§5). This file starts minimal for the intake tasks;
// lifecycle semantics (S1–S7) land with section 3.
package core

import "fmt"

// Sandbox is one sandbox record.
type Sandbox struct {
	Name          string `json:"name"`
	App           string `json:"app"`
	Owner         string `json:"owner"`
	Namespace     string `json:"namespace"`
	AppliedDigest string `json:"appliedDigest,omitempty"`
}

// CreateSandbox provisions a sandbox for an application, owned by the caller.
func (c *Core) CreateSandbox(appName, actor string) (*Sandbox, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.apps[appName]; !ok {
		return nil, errf(404, "application %q is not known", appName)
	}
	c.sandboxSeq++
	name := fmt.Sprintf("%s-sbx-%d", appName, c.sandboxSeq)
	sb := &Sandbox{
		Name:      name,
		App:       appName,
		Owner:     actor,
		Namespace: name,
	}
	c.sandboxes[name] = sb
	return sb, nil
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
