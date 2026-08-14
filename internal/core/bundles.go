// Bundles: the content-addressed unit of apply, diff, evidence, and promotion
// (B2, ADR 0002). The digest is computed over the rendered manifest bytes as
// submitted — recorded transforms never change it.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"cloudbox/internal/cluster"
)

// Bundle is a recorded, content-addressed manifest set.
type Bundle struct {
	Digest       string           `json:"digest"`
	App          string           `json:"app"`
	ManifestYAML string           `json:"-"`
	Objects      []cluster.Object `json:"-"`
	Transforms   []Transform      `json:"transforms"`
	Findings     []Finding        `json:"findings"`
}

// BundleDigest content-addresses rendered manifest bytes: the bytes that ran
// are the bytes that ship (ADR 0002).
func BundleDigest(manifestYAML string) string {
	sum := sha256.Sum256([]byte(manifestYAML))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ApplyOptions vary one apply.
type ApplyOptions struct {
	CapacityMode string // "" (squeezed default) | "squeezed" | "minimal" | "full"
	RecordEgress bool
}

// ApplyResult is what an apply reports back.
type ApplyResult struct {
	Digest     string      `json:"digest"`
	Findings   []Finding   `json:"findings"`
	Transforms []Transform `json:"transforms"`
}

// Apply renders intake end to end: parse (B1), analyze (B3/B4/C2/G7), reject
// on blockers naming manifest and fix, record the bundle (B2), transform for
// capacity (S7), admit workloads into the sealed namespace (N1), and update
// the run's evidence and soak clock (S6).
func (c *Core) Apply(appName, sandboxName, actor, manifestYAML string, opts ApplyOptions) (*ApplyResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	app, ok := c.apps[appName]
	if !ok {
		return nil, errf(404, "application %q is not known", appName)
	}
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return nil, errf(404, "sandbox %q is not known", sandboxName)
	}
	if sb.Owner != actor {
		return nil, errf(403, "sandbox %q is owned by %s: only the owner may modify it (S1)", sandboxName, sb.Owner)
	}
	if sb.State == "destroyed" {
		return nil, errf(409, "sandbox %q has been destroyed", sandboxName)
	}
	if !sb.Sealed {
		// N1: a sandbox is sealed before any user workload is admitted.
		return nil, errf(409,
			"sandbox %q is not sealed: no workload is admitted until default-deny ingress and egress are active (N1)",
			sandboxName)
	}
	sb.LastActivity = c.now()

	objects, err := ParseManifests(manifestYAML)
	if err != nil {
		return nil, err
	}
	analysis := AnalyzeManifests(objects, &app.Contract)

	// C2, second half: a declared secret must also have a value for the
	// target environment.
	declared := map[string]bool{}
	for _, s := range app.Contract.SecretNames {
		declared[s] = true
	}
	for _, obj := range objects {
		for _, secret := range collectSecretRefs(obj.Manifest) {
			if declared[secret] && !c.hasSecretValue(appName, "sandbox", secret) {
				analysis.Findings = append(analysis.Findings, Finding{
					Level:    "blocker",
					Code:     "secret-missing-value",
					Manifest: manifestID(obj),
					Message: fmt.Sprintf(
						"%s references secret %q, which is declared but has no value for the target environment %q; supply a value for that environment",
						manifestID(obj), secret, "sandbox"),
				})
			}
		}
	}

	// S7: capacity is a recorded, digest-preserving admission transform.
	mode := opts.CapacityMode
	if mode == "" {
		mode = "squeezed"
	}
	if mode != "squeezed" && mode != "minimal" && mode != "full" {
		return nil, errf(422, "unknown capacity mode %q (squeezed|minimal|full)", mode)
	}
	switch mode {
	case "squeezed":
		analysis.Transforms = append(analysis.Transforms, Transform{
			Kind:   "capacity",
			Detail: "mode squeezed (default): CPU requests scaled, memory floored per container, replica topology preserved",
		})
	case "minimal":
		analysis.Transforms = append(analysis.Transforms, Transform{
			Kind:   "capacity",
			Detail: "mode minimal: replicas floored to 1, requests scaled; N>1 behavior shifts to the canary",
		})
	}

	if blockers := analysis.Blockers(); len(blockers) > 0 {
		return nil, &Error{Status: 422, Message: blockers[0].Message, Findings: analysis.Findings}
	}

	// Build admitted manifests and meter the quota AFTER the transform (S5).
	host, hostKnown := c.driver.Cluster(sb.Cluster)
	var admitted []cluster.Object
	var suspended []string
	var totalCores float64
	for _, obj := range objects {
		if obj.Kind == "HorizontalPodAutoscaler" || obj.Kind == "VerticalPodAutoscaler" {
			if mode != "full" {
				// Autoscalers acting on transformed requests are suspended in
				// squeezed/minimal modes; the suspension is recorded (S7).
				suspended = append(suspended, manifestID(obj))
				continue
			}
		}
		manifest := applyCapacityTransform(obj, mode)
		if meta, ok := manifest["metadata"].(map[string]any); ok {
			meta["namespace"] = sb.Namespace // namespaces are assigned per environment (B3)
		}
		admitted = append(admitted, cluster.Object{
			APIVersion: obj.APIVersion, Kind: obj.Kind, Name: obj.Name,
			Namespace: sb.Namespace, Manifest: manifest,
		})
		totalCores += totalCPUCores(manifest)
	}
	if quota := app.Policies.CPUQuotaPerSandbox; quota > 0 && totalCores > quota {
		return nil, errf(422,
			"apply exceeds the application quota of %g CPUs per sandbox: bundle requests %g CPUs after the %s transform",
			quota, totalCores, mode)
	}

	bundle := &Bundle{
		Digest:       BundleDigest(manifestYAML),
		App:          appName,
		ManifestYAML: manifestYAML,
		Objects:      objects,
		Transforms:   analysis.Transforms,
		Findings:     analysis.Findings,
	}
	c.bundles[bundle.Digest] = bundle

	// Soak (S6): a digest change resets the clock; an identical digest
	// preserves accumulated soak (soak inheritance).
	if sb.AppliedDigest != bundle.Digest {
		if sb.AppliedDigest != "" {
			sb.InheritedSoak = 0
		}
		sb.SoakStart = c.now()
	}
	sb.AppliedDigest = bundle.Digest
	sb.CapacityMode = mode
	sb.SuspendedAutoscalers = suspended
	sb.Diagnostics = nil

	// Admit workloads into the sealed namespace.
	var migrationFailures []string
	if hostKnown {
		for _, obj := range admitted {
			if !isWorkloadKind(obj.Kind) {
				continue
			}
			w := cluster.Workload{Name: obj.Name, Ready: true, Manifest: obj.Manifest}
			if mode != "full" && c.oomArranger() != nil && c.oomArranger()(obj.Name) {
				// The workload cannot survive squeezing: surface it, never
				// hide it (S7).
				w.Ready = false
				w.OOMKilled = true
				sb.Diagnostics = append(sb.Diagnostics, Diagnostic{
					Code:     "capacity-squeeze-incompatible",
					Workload: obj.Name,
					Message: fmt.Sprintf(
						"workload %q was OOM-killed under the %s capacity transform; its memory floor is below what the workload needs — configure capacity: full for this application",
						obj.Name, mode),
				})
			}
			if isMigrationJob(obj) && c.migrationArranger() != nil && c.migrationArranger()(obj.Name) {
				// D4: the migration chain ran against the profile-schema
				// datastore and failed — surfaced in status and evidence.
				w.Ready = false
				failure := fmt.Sprintf("migration %q failed against the datastore instantiated from the profile schema", obj.Name)
				migrationFailures = append(migrationFailures, failure)
				sb.Diagnostics = append(sb.Diagnostics, Diagnostic{
					Code: "migration-failed", Workload: obj.Name, Message: failure,
				})
			}
			host.AddWorkload(sb.Namespace, w)
		}
	}

	// Evidence draft (G3 grows in section 5).
	ev := c.evidenceFor(sb)
	ev.BundleDigest = bundle.Digest
	ev.Transforms = append([]Transform{}, analysis.Transforms...)
	ev.CapacityMode = mode
	ev.AutoscalersSuspended = suspended
	ev.MigrationFailures = migrationFailures
	ev.SubstrateDigest = c.sandboxSubstrateDigest(sb)

	return &ApplyResult{
		Digest:     bundle.Digest,
		Findings:   analysis.Findings,
		Transforms: analysis.Transforms,
	}, nil
}

func isWorkloadKind(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Pod":
		return true
	}
	return false
}

// isMigrationJob spots schema-migration workloads: a Job whose name says so
// (the v1 heuristic behind D3's conditional fidelity rules).
func isMigrationJob(obj cluster.Object) bool {
	return obj.Kind == "Job" && strings.Contains(obj.Name, "migrat")
}

// bundleHasMigration reports whether a recorded bundle contains a migration.
// Callers hold c.mu.
func (c *Core) bundleHasMigration(digest string) bool {
	b, ok := c.bundles[digest]
	if !ok {
		return false
	}
	for _, obj := range b.Objects {
		if isMigrationJob(obj) {
			return true
		}
	}
	return false
}

// migrationArranger exposes the sim world's migration-failure arrangement.
// Callers hold c.mu.
func (c *Core) migrationArranger() func(string) bool {
	if w, ok := c.driver.(interface{ FailsMigration(string) bool }); ok {
		return w.FailsMigration
	}
	return nil
}

// oomArranger exposes the sim world's OOM arrangement when the driver is the
// sim; nil otherwise. Callers hold c.mu.
func (c *Core) oomArranger() func(string) bool {
	if w, ok := c.driver.(interface{ OOMsUnderSqueeze(string) bool }); ok {
		return w.OOMsUnderSqueeze
	}
	return nil
}

// ObservedHealthy computes the current soak for a sandbox (S6). Callers hold
// c.mu.
func (c *Core) observedHealthy(sb *Sandbox) time.Duration {
	if sb.SoakStart.IsZero() || sb.State != "ready" {
		return sb.InheritedSoak
	}
	return sb.InheritedSoak + c.now().Sub(sb.SoakStart)
}

// GetBundle returns the server-side record for a digest (B2).
func (c *Core) GetBundle(digest string) (*Bundle, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.bundles[digest]
	if !ok {
		return nil, errf(404, "no bundle recorded for digest %s", digest)
	}
	return b, nil
}
