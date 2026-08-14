// Diff (G2): compare a bundle against what production currently runs,
// normalized — defaulting, managed fields, and recorded intake transforms
// produce no diff lines.
package core

import (
	"fmt"
	"sort"

	"cloudbox/internal/cluster"
)

// ProductionState is what the team's own CD has running for an application.
// Below L3 the product only observes it, never writes it (G1).
type ProductionState struct {
	App     string
	Digest  string
	Objects []cluster.Object
}

// SetProductionState is a write to the application's production namespace by
// anything OTHER than an approved promotion: the team's own CD below L4, or a
// human with kubectl. Strict mode denies it unless the actor holds live
// break-glass credentials (G1/G12); every out-of-band write is checked for
// divergence.
func (c *Core) SetProductionState(appName, actor, manifestYAML string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[appName]
	if !ok {
		return errf(404, "application %q is not known", appName)
	}
	if app.Level == "L4" {
		if !c.hasBreakGlassLocked(appName, actor) {
			return errf(403,
				"write to managed production namespace %q denied by the product-managed RBAC: in strict mode production is writable only by the controller executing an approved promotion (G1); break-glass is the audited exception (G12)",
				appName)
		}
		c.recordAudit(actor, "break-glass-write", appName, "direct write under break-glass credentials")
	}
	if err := c.setProductionObjectsLocked(appName, manifestYAML); err != nil {
		return err
	}
	// G12: any production write outside an approved promotion is divergence.
	if promoted := c.promoted[appName]; promoted != nil && promoted.Digest != c.production[appName].Digest {
		promoted.Divergence = fmt.Sprintf(
			"production write outside an approved promotion: live digest %s no longer matches promoted %s",
			c.production[appName].Digest, promoted.Digest)
		promoted.EvidenceValid = false
		c.recordAudit("cloudbox-controller", "divergence-detected", appName, promoted.Divergence)
	}
	return nil
}

// setProductionObjectsLocked stores live production objects with the
// server-side noise a normalized diff must ignore. Callers hold c.mu.
func (c *Core) setProductionObjectsLocked(appName, manifestYAML string) error {
	objects, err := ParseManifests(manifestYAML)
	if err != nil {
		return err
	}
	for i := range objects {
		manifest := deepCopy(objects[i].Manifest)
		if meta, ok := manifest["metadata"].(map[string]any); ok {
			meta["creationTimestamp"] = "2026-01-01T00:00:00Z"
			meta["managedFields"] = []any{map[string]any{"manager": "argocd-controller"}}
			meta["namespace"] = appName
		}
		manifest["status"] = map[string]any{"observedGeneration": 4}
		objects[i].Manifest = manifest
	}
	c.production[appName] = &ProductionState{
		App: appName, Digest: BundleDigest(manifestYAML), Objects: objects,
	}
	return nil
}

// DiffLine is one normalized difference.
type DiffLine struct {
	Manifest string `json:"manifest"`
	Path     string `json:"path"`
	From     any    `json:"from"`
	To       any    `json:"to"`
}

// noiseFields never diff: server defaulting and bookkeeping (G2).
var noiseFields = map[string]bool{
	"creationTimestamp": true, "managedFields": true, "status": true,
	"generation": true, "resourceVersion": true, "uid": true, "namespace": true,
}

func normalizeForDiff(manifest map[string]any) map[string]any {
	out := deepCopy(manifest)
	var scrub func(any)
	scrub = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for key := range t {
				if noiseFields[key] {
					delete(t, key)
				}
			}
			for _, val := range t {
				scrub(val)
			}
		case []any:
			for _, item := range t {
				scrub(item)
			}
		}
	}
	scrub(out)
	return out
}

// DiffAgainstProduction compares a recorded bundle with production, normalized.
func (c *Core) DiffAgainstProduction(appName, digest string) ([]DiffLine, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.diffLocked(appName, digest)
}

func (c *Core) diffLocked(appName, digest string) ([]DiffLine, error) {
	bundle, ok := c.bundles[digest]
	if !ok {
		return nil, errf(404, "no bundle recorded for digest %s", digest)
	}
	prod, ok := c.production[appName]
	if !ok {
		return []DiffLine{{Manifest: "*", Path: "(new)", From: nil, To: digest}}, nil
	}

	prodByID := map[string]cluster.Object{}
	for _, obj := range prod.Objects {
		prodByID[manifestID(obj)] = obj
	}
	var lines []DiffLine
	for _, obj := range bundle.Objects {
		id := manifestID(obj)
		live, exists := prodByID[id]
		if !exists {
			lines = append(lines, DiffLine{Manifest: id, Path: "(added)"})
			continue
		}
		diffValue(id, "", normalizeForDiff(obj.Manifest), normalizeForDiff(live.Manifest), &lines)
		delete(prodByID, id)
	}
	for id := range prodByID {
		lines = append(lines, DiffLine{Manifest: id, Path: "(removed)"})
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Manifest+lines[i].Path < lines[j].Manifest+lines[j].Path
	})
	return lines, nil
}

func diffValue(id, path string, bundleV, liveV any, lines *[]DiffLine) {
	bm, bok := bundleV.(map[string]any)
	lm, lok := liveV.(map[string]any)
	if bok && lok {
		keys := map[string]bool{}
		for k := range bm {
			keys[k] = true
		}
		for k := range lm {
			keys[k] = true
		}
		for k := range keys {
			diffValue(id, path+"/"+k, bm[k], lm[k], lines)
		}
		return
	}
	if !equalValue(bundleV, liveV) {
		*lines = append(*lines, DiffLine{Manifest: id, Path: path, From: liveV, To: bundleV})
	}
}

func equalValue(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
