// Intake: parsing, analysis, and repair of incoming bundles (B1–B5, C2, and
// the determinism gate from G7). `check` (X3) is the offline advisory copy of
// this same analysis; the server-side run remains authoritative (CP2).
package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"cloudbox/internal/cluster"
)

// Finding is one intake analysis result. Blockers reject the apply; lint
// findings guide without blocking (the seal, not the lint, is the enforcement
// of last resort).
type Finding struct {
	Level      string `json:"level"` // "blocker" | "lint"
	Code       string `json:"code"`
	Manifest   string `json:"manifest"` // "Kind/name" of the violating manifest
	Message    string `json:"message"`  // names the fix
	Suggestion string `json:"suggestion,omitempty"`
}

// Transform is a recorded, digest-preserving normalization applied at
// admission (§5). Declared in evidence, never an edit to bundle bytes.
type Transform struct {
	Kind   string `json:"kind"` // "namespace" | "capacity"
	Detail string `json:"detail"`
}

// IntakeResult is everything analysis learned about a manifest set.
type IntakeResult struct {
	Objects    []cluster.Object `json:"-"`
	Findings   []Finding        `json:"findings"`
	Transforms []Transform      `json:"transforms"`
	// StrippedNamespace is the uniform namespace removed by the namespace
	// transform, empty if the bundle carried none.
	StrippedNamespace string `json:"strippedNamespace,omitempty"`
}

func (r *IntakeResult) Blockers() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Level == "blocker" {
			out = append(out, f)
		}
	}
	return out
}

// clusterScopedKinds are the built-in kinds intake recognizes as
// cluster-scoped: substrate, not bundle (B3).
var clusterScopedKinds = map[string]bool{
	"Namespace": true, "ClusterRole": true, "ClusterRoleBinding": true,
	"CustomResourceDefinition": true, "PriorityClass": true, "StorageClass": true,
	"IngressClass": true, "ValidatingWebhookConfiguration": true,
	"MutatingWebhookConfiguration": true, "PersistentVolume": true,
}

func manifestID(obj cluster.Object) string {
	if obj.Name == "" {
		return obj.Kind
	}
	return obj.Kind + "/" + obj.Name
}

// ParseManifests splits multi-document YAML into objects, accepting any kind,
// built-in or custom, with no product-specific schema (B1).
func ParseManifests(yamlText string) ([]cluster.Object, error) {
	dec := yaml.NewDecoder(strings.NewReader(yamlText))
	var objects []cluster.Object
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, errf(422, "manifest is not valid YAML: %v", err)
		}
		if doc == nil {
			continue
		}
		obj := cluster.FromManifest(doc)
		if obj.Kind == "" {
			return nil, errf(422, "a manifest document has no kind")
		}
		objects = append(objects, obj)
	}
	if len(objects) == 0 {
		return nil, errf(422, "no manifest documents found")
	}
	return objects, nil
}

// rfc3339ish spots wall-clock strings a deterministic render must not embed.
var rfc3339ish = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)

// uuidish spots random identifiers a deterministic render must not embed.
var uuidish = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// volatileAnnotationKey spots annotation keys that carry render-time values.
var volatileAnnotationKey = regexp.MustCompile(`(?i)(timestamp|generated-at|rendered-at|build-time|deploy-time|nonce|random)`)

func volatileValue(s string) bool {
	return rfc3339ish.MatchString(s) || uuidish.MatchString(s)
}

// AnalyzeManifests runs the full intake analysis over parsed objects.
// contract is nil when no boundary contract is available (offline check):
// contract-dependent rules are skipped there, which is exactly why the
// server-side run stays authoritative (CP2).
func AnalyzeManifests(objects []cluster.Object, contract *Contract) IntakeResult {
	res := IntakeResult{Objects: objects}

	// Cluster-scoped resources: rejected — substrate, not bundle (B3).
	for _, obj := range objects {
		if clusterScopedKinds[obj.Kind] {
			res.Findings = append(res.Findings, Finding{
				Level:    "blocker",
				Code:     "cluster-scoped-resource",
				Manifest: manifestID(obj),
				Message: fmt.Sprintf(
					"%s is cluster-scoped: cluster-scoped resources belong to the substrate, not the bundle; move it to the application's substrate (or the future virtual-cluster mechanism) and remove it from the bundle",
					manifestID(obj)),
			})
		}
	}

	// Namespace uniformity (B3): one uniform namespace is stripped by a
	// recorded transform; multiple distinct namespaces fail with guidance.
	nsSet := map[string]bool{}
	var nsOffenders []string
	for _, obj := range objects {
		if obj.Namespace != "" && !clusterScopedKinds[obj.Kind] {
			nsSet[obj.Namespace] = true
			nsOffenders = append(nsOffenders, manifestID(obj)+" (namespace "+obj.Namespace+")")
		}
	}
	switch {
	case len(nsSet) == 1:
		var ns string
		for k := range nsSet {
			ns = k
		}
		res.StrippedNamespace = ns
		res.Transforms = append(res.Transforms, Transform{
			Kind:   "namespace",
			Detail: fmt.Sprintf("stripped uniform namespace %q; namespaces are assigned per environment", ns),
		})
	case len(nsSet) > 1:
		names := make([]string, 0, len(nsSet))
		for k := range nsSet {
			names = append(names, k)
		}
		sort.Strings(names)
		res.Findings = append(res.Findings, Finding{
			Level:    "blocker",
			Code:     "multi-namespace",
			Manifest: strings.Join(nsOffenders, ", "),
			Message: fmt.Sprintf(
				"bundle spans namespaces %s: an application occupies exactly one namespace per environment; split into one application per namespace, or use the multi-namespace (virtual-cluster) path when available",
				strings.Join(names, ", ")),
		})
	}

	// Service references (B3/B4): scan every string for http(s) URLs.
	for _, obj := range objects {
		for _, ref := range collectURLHosts(obj.Manifest) {
			classifyReference(&res, obj, ref, contract)
		}
	}

	// Determinism (G7): deterministic renders embed no wall-clock values.
	for _, obj := range objects {
		if meta, ok := obj.Manifest["metadata"].(map[string]any); ok {
			if ann, ok := meta["annotations"].(map[string]any); ok {
				for key, val := range ann {
					sval, _ := val.(string)
					if volatileAnnotationKey.MatchString(key) || volatileValue(sval) {
						res.Findings = append(res.Findings, Finding{
							Level:    "blocker",
							Code:     "non-deterministic-render",
							Manifest: manifestID(obj),
							Message: fmt.Sprintf(
								"determinism error: annotation %q on %s embeds a render-time value (%q); renders must be reproducible — pin charts and values and drop timestamps and randomness so the digest cannot flap",
								key, manifestID(obj), sval),
						})
					}
				}
			}
		}
	}

	// Secrets (C2): every referenced secret must be declared in the contract.
	// Contract-dependent, so skipped when analysis runs offline.
	if contract != nil {
		declared := map[string]bool{}
		for _, name := range contract.SecretNames {
			declared[name] = true
		}
		for _, obj := range objects {
			for _, secret := range collectSecretRefs(obj.Manifest) {
				if !declared[secret] {
					res.Findings = append(res.Findings, Finding{
						Level:    "blocker",
						Code:     "undeclared-secret",
						Manifest: manifestID(obj),
						Message: fmt.Sprintf(
							"%s references secret %q, which is not declared in the boundary contract; declare it under the contract's secret names and supply a value per environment",
							manifestID(obj), secret),
					})
				}
			}
		}
	}

	return res
}

type urlRef struct {
	url  string
	host string
}

var urlPattern = regexp.MustCompile(`https?://([A-Za-z0-9.-]+)(:\d+)?`)

func collectURLHosts(v any) []urlRef {
	var refs []urlRef
	walkStrings(v, func(s string) {
		for _, m := range urlPattern.FindAllStringSubmatch(s, -1) {
			refs = append(refs, urlRef{url: m[0], host: m[1]})
		}
	})
	return refs
}

func classifyReference(res *IntakeResult, obj cluster.Object, ref urlRef, contract *Contract) {
	host := ref.host
	if !strings.Contains(host, ".") {
		return // same-namespace short name (B4): fine
	}
	if contract != nil {
		for _, dep := range contract.Dependencies {
			if dep.Alias == host {
				return // declared boundary contract alias (B4/C1): fine
			}
		}
		for _, fqdn := range contract.EgressAllowlist {
			if fqdn == host {
				return // declared external endpoint: the seal governs it
			}
		}
	}
	// A dotted host that is neither a declared alias nor allowlisted external:
	// best-effort lint with a suggested rewrite (B3). The seal, not the lint,
	// is the enforcement of last resort.
	short := strings.SplitN(host, ".", 2)[0]
	res.Findings = append(res.Findings, Finding{
		Level:    "lint",
		Code:     "cross-namespace-reference",
		Manifest: manifestID(obj),
		Message: fmt.Sprintf(
			"%s references %q, which crosses the application namespace; in-bundle references use same-namespace short names or declared contract aliases",
			manifestID(obj), ref.url),
		Suggestion: fmt.Sprintf("rewrite %q to the same-namespace short name %q, or declare an alias in the boundary contract", ref.url, short),
	})
}

func collectSecretRefs(v any) []string {
	var names []string
	var walk func(any)
	walk = func(node any) {
		switch t := node.(type) {
		case map[string]any:
			// env valueFrom secretKeyRef.name, envFrom secretRef.name,
			// volumes[].secret.secretName
			if ref, ok := t["secretKeyRef"].(map[string]any); ok {
				if n, ok := ref["name"].(string); ok {
					names = append(names, n)
				}
			}
			if ref, ok := t["secretRef"].(map[string]any); ok {
				if n, ok := ref["name"].(string); ok {
					names = append(names, n)
				}
			}
			if ref, ok := t["secret"].(map[string]any); ok {
				if n, ok := ref["secretName"].(string); ok {
					names = append(names, n)
				}
			}
			for _, val := range t {
				walk(val)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(v)
	return names
}

func walkStrings(v any, visit func(string)) {
	switch t := v.(type) {
	case string:
		visit(t)
	case map[string]any:
		for _, val := range t {
			walkStrings(val, visit)
		}
	case []any:
		for _, item := range t {
			walkStrings(item, visit)
		}
	}
}
