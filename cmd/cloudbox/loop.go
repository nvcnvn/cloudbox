// The sealed-iteration loop at the sandbox level (developer-ergonomics): with
// no source-control integration, the command line carries every step —
// register a host, declare an application and its contract, create a sandbox,
// apply, run the declared suite, read status and evidence, amend, destroy.
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// runCluster: cloudbox cluster register <name> [--production]
func runCluster(c *client, args []string) int {
	const use = "cloudbox cluster register <kube-context> [--production]"
	if len(args) < 2 || args[0] != "register" || strings.HasPrefix(args[1], "-") {
		return c.usage("cluster", use)
	}
	name := args[1]
	if hasFlag(args, "--production") {
		out, f := c.call("POST", "/v1/clusters/register", map[string]any{"cluster": name, "role": "production"})
		if f != nil {
			return c.fail("cluster register", f)
		}
		return c.emit(pick((&view{}).add("cluster", name), out, "role", "status"))
	}
	if _, f := c.call("POST", "/v1/setup", map[string]any{"cluster": name}); f != nil {
		return c.fail("cluster register", f)
	}
	// Every host registered from the command line is one the caller
	// controls (ADR 0010): its evidence is never postable or promotable.
	out, f := c.call("POST", "/v1/clusters/register",
		map[string]any{"cluster": name, "role": "sandbox", "userControlled": true})
	if f != nil {
		return c.fail("cluster register", f)
	}
	return c.emit(pick((&view{}).add("cluster", name), out, "role", "status").add("userControlled", true))
}

// runApp: declare an application and its contract, or amend its allowlist.
func runApp(c *client, args []string) int {
	const use = "cloudbox app create <name> [--allow <fqdn>]... [--secret <name>]... [--suite <name> --tests <n>]\n" +
		"       cloudbox app allow <name> <fqdn>...\n" +
		"       cloudbox app show <name>"
	if len(args) < 2 || strings.HasPrefix(args[1], "-") {
		return c.usage("app", use)
	}
	name := args[1]
	switch args[0] {
	case "create":
		app := map[string]any{
			"name":   name,
			"owners": []string{c.identity()},
			"contract": map[string]any{
				"egressAllowlist": nonNil(flagValues(args, "--allow")),
				"secretNames":     nonNil(flagValues(args, "--secret")),
			},
		}
		if suite := flagValue(args, "--suite"); suite != "" {
			tests, _ := strconv.Atoi(flagValue(args, "--tests"))
			app["testSuite"] = map[string]any{"name": suite, "tests": tests}
		}
		out, f := c.call("POST", "/v1/applications", app)
		if f != nil {
			return c.fail("app create", f)
		}
		return c.emit(appView(out))
	case "allow":
		fqdns := args[2:]
		if len(fqdns) == 0 {
			return c.usage("app", use)
		}
		current, f := c.call("GET", "/v1/applications/"+name, nil)
		if f != nil {
			return c.fail("app allow", f)
		}
		allow := map[string]bool{}
		if contract, ok := current["contract"].(map[string]any); ok {
			for _, d := range asStrings(contract["egressAllowlist"]) {
				allow[d] = true
			}
		}
		for _, d := range fqdns {
			allow[d] = true
		}
		list := make([]string, 0, len(allow))
		for d := range allow {
			list = append(list, d)
		}
		sort.Strings(list)
		out, f := c.call("PUT", "/v1/applications/"+name+"/allowlist", map[string]any{"allowlist": list})
		if f != nil {
			return c.fail("app allow", f)
		}
		return c.emit(pick((&view{}).add("app", name), out, "allowlist"))
	case "show":
		out, f := c.call("GET", "/v1/applications/"+name, nil)
		if f != nil {
			return c.fail("app show", f)
		}
		return c.emit(appView(out))
	}
	return c.usage("app", use)
}

func appView(out map[string]any) *view {
	return pick(&view{}, out, "name", "owners", "level", "contract", "testSuite", "sandboxCluster")
}

// runSandbox: cloudbox sandbox create -a <app> [--ttl <seconds>] | destroy -s <sandbox>
func runSandbox(c *client, args []string) int {
	const use = "cloudbox sandbox create -a <app> [--ttl <seconds>]\n       cloudbox sandbox destroy -s <sandbox>"
	if len(args) == 0 {
		return c.usage("sandbox", use)
	}
	switch args[0] {
	case "create":
		app := flagValue(args, "-a")
		if app == "" {
			return c.usage("sandbox", use)
		}
		body := map[string]any{"app": app}
		if ttl, err := strconv.Atoi(flagValue(args, "--ttl")); err == nil && ttl > 0 {
			body["ttlSeconds"] = ttl
		}
		out, f := c.call("POST", "/v1/sandboxes", body)
		if f != nil {
			return c.fail("sandbox create", f)
		}
		return c.emit(pick(&view{}, out, "name", "app", "cluster", "state", "sealed", "sealVerified", "userControlled"))
	case "destroy":
		sandbox := flagValue(args, "-s")
		if sandbox == "" {
			return c.usage("sandbox", use)
		}
		if _, f := c.call("DELETE", "/v1/sandboxes/"+sandbox, nil); f != nil {
			return c.fail("sandbox destroy", f)
		}
		return c.emit((&view{}).add("sandbox", sandbox).add("destroyed", true))
	}
	return c.usage("sandbox", use)
}

// runApply: cloudbox apply -a <app> -s <sandbox> -f <file-or-dir> [--record-egress]
func runApply(c *client, args []string) int {
	const use = "cloudbox apply -a <app> -s <sandbox> -f <file-or-dir> [--record-egress]"
	if bad := unknownFlag(args, []string{"-a", "-s", "-f"}, []string{"--record-egress"}); bad != "" {
		return c.usage("apply", "unknown flag "+bad+"\n       "+use)
	}
	app, sandbox, path := flagValue(args, "-a"), flagValue(args, "-s"), flagValue(args, "-f")
	if app == "" || sandbox == "" || path == "" {
		return c.usage("apply", use)
	}
	manifests, err := readManifests(path)
	if err != nil {
		return c.fail("apply", &failure{exit: exitUsage, message: err.Error()})
	}
	out, f := c.call("POST", "/v1/apply", map[string]any{
		"app": app, "sandbox": sandbox, "manifests": manifests,
		"recordEgress": hasFlag(args, "--record-egress"),
	})
	if f != nil {
		// Intake rejections carry their findings; show them, not just the summary.
		if findings, ok := out["findings"]; ok && c.output != "json" {
			f.message += "\n" + humanValue(findings)
		}
		return c.fail("apply", f)
	}
	return c.emit(pick((&view{}).add("sandbox", sandbox), out, "digest", "findings", "transforms"))
}

// readManifests reads a YAML file, or every .yaml/.yml file in a directory in
// name order, as one multi-document stream.
func readManifests(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		raw, err := os.ReadFile(path)
		return string(raw), err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	var docs []string
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(path, e.Name()))
		if err != nil {
			return "", err
		}
		docs = append(docs, strings.TrimSpace(string(raw)))
	}
	return strings.Join(docs, "\n---\n"), nil
}

// runTest: cloudbox test -s <sandbox> [--suite <name>]
func runTest(c *client, args []string) int {
	sandbox := flagValue(args, "-s")
	if sandbox == "" {
		return c.usage("test", "cloudbox test -s <sandbox> [--suite <name>]")
	}
	out, f := c.call("POST", "/v1/sandboxes/"+sandbox+"/test", map[string]any{"suite": flagValue(args, "--suite")})
	if f != nil {
		return c.fail("test", f)
	}
	return c.emit(pick((&view{}).add("sandbox", sandbox), out, "suite", "passed", "total", "at"))
}

// runEvidence: cloudbox evidence -s <sandbox>
func runEvidence(c *client, args []string) int {
	sandbox := flagValue(args, "-s")
	if sandbox == "" {
		return c.usage("evidence", "cloudbox evidence -s <sandbox>")
	}
	out, f := c.call("GET", "/v1/sandboxes/"+sandbox+"/evidence", nil)
	if f != nil {
		return c.fail("evidence", f)
	}
	return c.emit(pick(&view{}, out,
		"sandbox", "source", "bundleDigest", "summary", "valid", "invalidReasons",
		"sealStatus", "egressViolations", "egressRecordIncomplete", "egressRecordGap",
		"substrateDigest", "substrateParity", "fidelity", "capacityMode", "readiness",
		"observedHealthySeconds", "witnessed", "dependencies", "signature"))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func asStrings(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, i := range items {
		if s, ok := i.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
