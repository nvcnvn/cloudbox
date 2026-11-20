// Sealed-life verbs (X1/X2): status --explain, logs, exec, port-forward.
package main

import (
	"fmt"
)

// runStatus renders sandbox readiness, and with --explain every blocked
// egress attempt plus the ready-to-submit allowlist proposal (X2). Both
// renderings project one view, so they carry the same facts (design D7).
func runStatus(c *client, args []string) int {
	sandbox := flagValue(args, "-s")
	if sandbox == "" {
		return c.usage("status", "cloudbox status -s <sandbox> [--explain]")
	}
	record, f := c.call("GET", "/v1/sandboxes/"+sandbox, nil)
	if f != nil {
		return c.fail("status", f)
	}
	v := (&view{}).add("sandbox", sandbox)
	pick(v, record, "state", "sealed", "capacityMode", "userControlled")
	diagnostics := []any{}
	if ds, ok := record["diagnostics"].([]any); ok {
		for _, d := range ds {
			diag, _ := d.(map[string]any)
			diagnostics = append(diagnostics, map[string]any{
				"code": diag["code"], "workload": diag["workload"], "message": diag["message"],
			})
		}
	}
	v.add("diagnostics", diagnostics)

	explain := hasFlag(args, "--explain")
	if explain {
		ex, f := c.call("GET", "/v1/sandboxes/"+sandbox+"/explain", nil)
		if f != nil {
			return c.fail("status", f)
		}
		blocked := []any{}
		if bs, ok := ex["blockedEgress"].([]any); ok {
			for _, b := range bs {
				a, _ := b.(map[string]any)
				blocked = append(blocked, map[string]any{
					"destination": a["destination"], "at": a["at"], "workload": a["workload"],
				})
			}
		}
		v.add("blockedEgress", blocked)
		if p, ok := ex["proposal"].(map[string]any); ok {
			v.add("proposal", map[string]any{
				"app": p["app"], "addFqdns": p["addFqdns"], "submitTo": p["submitTo"],
			})
		} else {
			v.add("proposal", nil)
		}
	}

	v.human = func(v *view) {
		fmt.Printf("sandbox %s  state=%v sealed=%v capacity=%v userControlled=%v\n",
			sandbox, v.get("state"), v.get("sealed"), v.get("capacityMode"), v.get("userControlled"))
		for _, d := range v.get("diagnostics").([]any) {
			diag := d.(map[string]any)
			fmt.Printf("DIAGNOSTIC %v %v: %v\n", diag["code"], diag["workload"], diag["message"])
		}
		if !explain {
			return
		}
		for _, b := range v.get("blockedEgress").([]any) {
			a := b.(map[string]any)
			fmt.Printf("BLOCKED egress to %v at %v from workload %v\n", a["destination"], a["at"], a["workload"])
		}
		if p, ok := v.get("proposal").(map[string]any); ok {
			fmt.Printf("\nproposed allowlist change for application %v (ready to submit for %v):\n", p["app"], p["submitTo"])
			for _, fqdn := range asStrings(p["addFqdns"]) {
				fmt.Printf("  + %v\n", fqdn)
			}
		}
	}
	return c.emit(v)
}

func runLogs(c *client, args []string) int {
	sandbox, workload := flagValue(args, "-s"), flagValue(args, "-w")
	if sandbox == "" || workload == "" {
		return c.usage("logs", "cloudbox logs -s <sandbox> -w <workload>")
	}
	out, f := c.call("GET", "/v1/sandboxes/"+sandbox+"/workloads/"+workload+"/logs", nil)
	if f != nil {
		return c.fail("logs", f)
	}
	v := (&view{}).add("sandbox", sandbox).add("workload", workload).add("logs", out["logs"])
	v.human = func(v *view) { fmt.Print(v.get("logs")) }
	return c.emit(v)
}

func runExec(c *client, args []string) int {
	sandbox, workload := flagValue(args, "-s"), flagValue(args, "-w")
	command := flagValue(args, "--")
	if command == "" {
		command = flagValue(args, "-c")
	}
	if sandbox == "" || workload == "" || command == "" {
		return c.usage("exec", "cloudbox exec -s <sandbox> -w <workload> -c <command>")
	}
	out, f := c.call("POST", "/v1/sandboxes/"+sandbox+"/exec",
		map[string]any{"workload": workload, "command": command})
	if f != nil {
		return c.fail("exec", f)
	}
	v := (&view{}).add("sandbox", sandbox).add("workload", workload).add("output", out["output"])
	v.human = func(v *view) { fmt.Print(v.get("output")) }
	return c.emit(v)
}

func runPortForward(c *client, args []string) int {
	sandbox, workload := flagValue(args, "-s"), flagValue(args, "-w")
	if sandbox == "" || workload == "" {
		return c.usage("port-forward", "cloudbox port-forward -s <sandbox> -w <workload>")
	}
	out, f := c.call("POST", "/v1/sandboxes/"+sandbox+"/port-forward",
		map[string]any{"workload": workload, "port": 8080})
	if f != nil {
		return c.fail("port-forward", f)
	}
	v := (&view{}).add("sandbox", sandbox).add("workload", workload).
		add("via", out["via"]).add("localUrl", out["localUrl"])
	v.human = func(v *view) { fmt.Printf("forwarding via %v: %v\n", v.get("via"), v.get("localUrl")) }
	return c.emit(v)
}
