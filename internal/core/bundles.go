// Bundles: the content-addressed unit of apply, diff, evidence, and promotion
// (B2, ADR 0002). The digest is computed over the rendered manifest bytes as
// submitted — recorded transforms never change it.
package core

import (
	"crypto/sha256"
	"encoding/hex"

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
