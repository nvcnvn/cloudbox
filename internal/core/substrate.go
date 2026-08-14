// Substrate parity (P1–P4, ADR 0006): the lockfile is scoped to what the
// application's bundles actually reference, so drift invalidates only the
// applications that depend on the drifted component.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"cloudbox/internal/cluster"
)

// Lockfile is the per-application substrate lockfile (P1).
type Lockfile struct {
	App             string              `json:"app"`
	KubernetesMinor string              `json:"kubernetesMinor"`
	Components      []cluster.Component `json:"components"` // referenced operators + admission
	Classes         []string            `json:"classes"`    // referenced named classes
	// DeclaredNotVerified lists what the seal cannot prove: cloud identity
	// bindings and secret values (P1, §2.4).
	DeclaredNotVerified []string `json:"declaredNotVerified"`
	Digest              string   `json:"digest"`
}

// referencedGroups extracts the API groups of custom resources the app's
// bundles instantiate, and the class names they name. Callers hold c.mu.
func (c *Core) referencedSubstrate(appName string) (groups map[string]bool, classes map[string]bool, declared []string) {
	groups = map[string]bool{}
	classes = map[string]bool{}
	builtin := map[string]bool{"": true, "apps": true, "batch": true, "v1": true,
		"networking.k8s.io": true, "rbac.authorization.k8s.io": true, "autoscaling": true}
	for _, b := range c.bundles {
		if !c.bundleBelongsTo(b, appName) {
			continue
		}
		for _, obj := range b.Objects {
			group := ""
			if i := strings.Index(obj.APIVersion, "/"); i >= 0 {
				group = obj.APIVersion[:i]
			}
			if !builtin[group] {
				groups[group] = true
			}
			walkStrings(obj.Manifest, func(s string) {})
			collectClassNames(obj.Manifest, classes)
			declared = append(declared, collectDeclaredNotVerified(obj.Manifest)...)
		}
	}
	sort.Strings(declared)
	return groups, classes, declared
}

// bundleBelongsTo reports whether a bundle was applied for this application.
// Callers hold c.mu.
func (c *Core) bundleBelongsTo(b *Bundle, appName string) bool {
	for _, sb := range c.sandboxes {
		if sb.App == appName && sb.AppliedDigest == b.Digest {
			return true
		}
	}
	return b.App == appName
}

func collectClassNames(v any, classes map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		for key, val := range t {
			if sval, ok := val.(string); ok {
				switch key {
				case "storageClassName", "ingressClassName", "priorityClassName", "gatewayClassName":
					classes[sval] = true
				}
			}
			collectClassNames(val, classes)
		}
	case []any:
		for _, item := range t {
			collectClassNames(item, classes)
		}
	}
}

// collectDeclaredNotVerified finds cloud identity bindings the seal cannot
// verify: the workload reached an endpoint, never that production credentials
// are authorized there (P1).
func collectDeclaredNotVerified(v any) []string {
	var out []string
	switch t := v.(type) {
	case map[string]any:
		for key, val := range t {
			if key == "annotations" {
				if ann, ok := val.(map[string]any); ok {
					for akey := range ann {
						if strings.Contains(akey, "role-arn") || strings.Contains(akey, "workload-identity") ||
							strings.Contains(akey, "iam.gke.io") || strings.Contains(akey, "azure.workload.identity") {
							out = append(out, "identity-binding:"+akey+" (declared-not-verified)")
						}
					}
				}
			}
			out = append(out, collectDeclaredNotVerified(val)...)
		}
	case []any:
		for _, item := range t {
			out = append(out, collectDeclaredNotVerified(item)...)
		}
	}
	return out
}

// productionCluster resolves the registered production cluster. Callers hold
// c.mu.
func (c *Core) productionCluster() (cluster.Cluster, bool) {
	for name, roles := range c.roles {
		if roles["production"] {
			return c.driver.Cluster(name)
		}
	}
	return nil, false
}

// lockfileFor computes the application-scoped substrate lockfile against a
// cluster's installed substrate (P1). Callers hold c.mu.
func (c *Core) lockfileFor(appName string, cl cluster.Cluster) *Lockfile {
	groups, classNames, declared := c.referencedSubstrate(appName)

	lf := &Lockfile{
		App:             appName,
		KubernetesMinor: cl.KubernetesMinor(),
		Components:      []cluster.Component{},
		Classes:         []string{},
	}
	for _, comp := range cl.Components() {
		switch comp.Kind {
		case "operator":
			for _, g := range comp.OwnsCRDs {
				if groups[g] {
					lf.Components = append(lf.Components, comp)
					break
				}
			}
		case "admission":
			// Admission configurations apply to everything admitted into the
			// namespace, so applicable ones are always in scope.
			lf.Components = append(lf.Components, comp)
		case "class":
			for _, class := range comp.Classes {
				if classNames[class] {
					lf.Classes = append(lf.Classes, class)
				}
			}
		}
	}
	sort.Slice(lf.Components, func(i, j int) bool { return lf.Components[i].Name < lf.Components[j].Name })
	sort.Strings(lf.Classes)

	// Secret values are per-environment and never verified by the seal.
	app := c.apps[appName]
	if app != nil {
		for _, s := range app.Contract.SecretNames {
			declared = append(declared, "secret-value:"+s+" (declared-not-verified)")
		}
	}
	sort.Strings(declared)
	lf.DeclaredNotVerified = declared

	blob, _ := json.Marshal(struct {
		Minor      string
		Components []cluster.Component
		Classes    []string
	}{lf.KubernetesMinor, lf.Components, lf.Classes})
	sum := sha256.Sum256(blob)
	lf.Digest = "sha256:" + hex.EncodeToString(sum[:])
	return lf
}

// SubstrateLockfile returns the application's lockfile computed against the
// registered production cluster (P1, P4: recomputing after drift moves the
// digest for referencing applications only).
func (c *Core) SubstrateLockfile(appName string) (*Lockfile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.apps[appName]; !ok {
		return nil, errf(404, "application %q is not known", appName)
	}
	prod, ok := c.productionCluster()
	if !ok {
		return nil, errf(409, "no production cluster registered")
	}
	return c.lockfileFor(appName, prod), nil
}

// sandboxSubstrateDigest computes the app-scoped digest of the sandbox's own
// cluster (what the sandbox actually ran on). Callers hold c.mu.
func (c *Core) sandboxSubstrateDigest(sb *Sandbox) string {
	host, ok := c.driver.Cluster(sb.Cluster)
	if !ok {
		return ""
	}
	return c.lockfileFor(sb.App, host).Digest
}

// provisionLocalCluster creates a user-controlled local cluster provisioned
// from the application's substrate lockfile (S3, P3). Callers hold c.mu.
func (c *Core) provisionLocalCluster(sandboxName, appName string) (cluster.Cluster, error) {
	creator, ok := c.driver.(interface {
		NewCluster(name string, enforcing bool, userControlled bool) cluster.Cluster
	})
	if !ok {
		return nil, errf(500, "driver cannot provision local clusters")
	}
	local := creator.NewCluster("kind-"+sandboxName, true, true)
	if prod, ok := c.productionCluster(); ok {
		local.SetComponents(prod.KubernetesMinor(), prod.Components())
	}
	return local, nil
}

// ProvisionSandboxSubstrate makes a sandbox host provide the lockfile's
// components (P3: provisioning is the convenience that makes verification
// pass). Callers hold c.mu.
func (c *Core) provisionSandboxSubstrate(host cluster.Cluster) {
	if prod, ok := c.productionCluster(); ok && prod.Name() != host.Name() {
		host.SetComponents(prod.KubernetesMinor(), prod.Components())
	}
}
