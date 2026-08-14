// Bundles: the content-addressed unit of apply, diff, evidence, and promotion
// (B2, ADR 0002). The digest is computed over the rendered manifest bytes as
// submitted — recorded transforms never change it.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"cloudbox/internal/cluster"
)

// Bundle is a recorded, content-addressed manifest set.
type Bundle struct {
	Digest       string      `json:"digest"`
	ManifestYAML string      `json:"-"`
	Objects      []cluster.Object `json:"-"`
	Transforms   []Transform `json:"transforms"`
	Findings     []Finding   `json:"findings"`
}

// BundleDigest content-addresses rendered manifest bytes: the bytes that ran
// are the bytes that ship (ADR 0002).
func BundleDigest(manifestYAML string) string {
	sum := sha256.Sum256([]byte(manifestYAML))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ApplyResult is what an apply reports back.
type ApplyResult struct {
	Digest     string      `json:"digest"`
	Findings   []Finding   `json:"findings"`
	Transforms []Transform `json:"transforms"`
}

// Apply renders intake end to end: parse (B1), analyze (B3/B4/C2/G7), reject
// on blockers naming manifest and fix, record the bundle (B2), and record the
// run's transforms in the sandbox's evidence draft.
func (c *Core) Apply(appName, sandboxName, actor, manifestYAML string) (*ApplyResult, error) {
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

	objects, err := ParseManifests(manifestYAML)
	if err != nil {
		return nil, err
	}
	analysis := AnalyzeManifests(objects, &app.Contract)

	// C2, second half: a declared secret must also have a value for the
	// target environment. Environment-value state lives server-side, which is
	// one more reason the offline check stays advisory (CP2).
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

	// S7: capacity is a recorded, digest-preserving admission transform,
	// declared in evidence — default mode squeezed. Full squeeze semantics
	// land with the sandbox lifecycle capability.
	analysis.Transforms = append(analysis.Transforms, Transform{
		Kind:   "capacity",
		Detail: "mode squeezed (default): CPU requests scaled, memory floored per container, replica topology preserved",
	})

	if blockers := analysis.Blockers(); len(blockers) > 0 {
		return nil, &Error{Status: 422, Message: blockers[0].Message, Findings: analysis.Findings}
	}

	bundle := &Bundle{
		Digest:       BundleDigest(manifestYAML),
		ManifestYAML: manifestYAML,
		Objects:      objects,
		Transforms:   analysis.Transforms,
		Findings:     analysis.Findings,
	}
	c.bundles[bundle.Digest] = bundle
	sb.AppliedDigest = bundle.Digest

	ev := c.evidenceFor(sb)
	ev.BundleDigest = bundle.Digest
	ev.Transforms = append([]Transform{}, analysis.Transforms...)

	return &ApplyResult{
		Digest:     bundle.Digest,
		Findings:   analysis.Findings,
		Transforms: analysis.Transforms,
	}, nil
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
