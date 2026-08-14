// Sealed-life verbs (X1/X2): status --explain, logs, exec, port-forward.
package main

import (
	"fmt"
	"os"
)

// runStatus renders sandbox readiness, and with --explain every blocked
// egress attempt plus the ready-to-submit allowlist proposal (X2).
func runStatus(c *client, args []string) int {
	sandbox := flagValue(args, "-s")
	if sandbox == "" {
		fmt.Fprintln(os.Stderr, "usage: cloudbox status -a <app> -s <sandbox> [--explain]")
		return 2
	}
	record, status := c.do("GET", "/v1/sandboxes/"+sandbox, nil)
	if status >= 400 {
		fmt.Fprintf(os.Stderr, "status: %v\n", record["error"])
		return 1
	}
	fmt.Printf("sandbox %s  state=%v sealed=%v capacity=%v\n",
		sandbox, record["state"], record["sealed"], record["capacityMode"])
	if diagnostics, ok := record["diagnostics"].([]any); ok {
		for _, d := range diagnostics {
			diag := d.(map[string]any)
			fmt.Printf("DIAGNOSTIC %v %v: %v\n", diag["code"], diag["workload"], diag["message"])
		}
	}
	if !hasFlag(args, "--explain") {
		return 0
	}

	explain, status := c.do("GET", "/v1/sandboxes/"+sandbox+"/explain", nil)
	if status >= 400 {
		fmt.Fprintf(os.Stderr, "explain: %v\n", explain["error"])
		return 1
	}
	blocked, _ := explain["blockedEgress"].([]any)
	for _, b := range blocked {
		attempt := b.(map[string]any)
		fmt.Printf("BLOCKED egress to %v at %v from workload %v\n",
			attempt["destination"], attempt["at"], attempt["workload"])
	}
	if proposal, ok := explain["proposal"].(map[string]any); ok {
		fmt.Printf("\nproposed allowlist change for application %v (ready to submit for %v):\n",
			proposal["app"], proposal["submitTo"])
		for _, fqdn := range proposal["addFqdns"].([]any) {
			fmt.Printf("  + %v\n", fqdn)
		}
	}
	return 0
}

func runLogs(c *client, args []string) int {
	sandbox, workload := flagValue(args, "-s"), flagValue(args, "-w")
	out, status := c.do("GET", "/v1/sandboxes/"+sandbox+"/workloads/"+workload+"/logs", nil)
	if status >= 400 {
		fmt.Fprintf(os.Stderr, "logs: %v\n", out["error"])
		return 1
	}
	fmt.Print(out["logs"])
	return 0
}

func runExec(c *client, args []string) int {
	sandbox, workload := flagValue(args, "-s"), flagValue(args, "-w")
	command := flagValue(args, "--")
	if command == "" {
		command = flagValue(args, "-c")
	}
	out, status := c.do("POST", "/v1/sandboxes/"+sandbox+"/exec",
		map[string]any{"workload": workload, "command": command})
	if status >= 400 {
		fmt.Fprintf(os.Stderr, "exec: %v\n", out["error"])
		return 1
	}
	fmt.Print(out["output"])
	return 0
}

func runPortForward(c *client, args []string) int {
	sandbox, workload := flagValue(args, "-s"), flagValue(args, "-w")
	out, status := c.do("POST", "/v1/sandboxes/"+sandbox+"/port-forward",
		map[string]any{"workload": workload, "port": 8080})
	if status >= 400 {
		fmt.Fprintf(os.Stderr, "port-forward: %v\n", out["error"])
		return 1
	}
	fmt.Printf("forwarding via %v: %v\n", out["via"], out["localUrl"])
	return 0
}
