// cloudbox is the thin CLI client (ADR 0004): it creates resources and watches
// status via the control plane; the one offline exception is `check` (X3).
// With no control plane addressed it starts a local one and reuses it; it
// never becomes one.
package main

import (
	"fmt"
	"os"

	"cloudbox/internal/release"
)

// version identifies the build (release-integrity), stamped by the release
// process; anything else reports itself as unreleased.
var version = release.Version

const usage = `cloudbox — sealed, production-shaped sandboxes with honest evidence

Usage:
  cloudbox [--server <url>] [--as <identity>] [-o json] <command> [flags]

With no --server, cloudbox starts a local control plane and reuses it.
Run a command without flags for its own usage.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// Global flags: --server addresses a control plane (it always wins over
	// a local one); --as is the identity the request asserts; -o selects
	// the rendering.
	c := &client{output: "human", ensure: ensureLocal}
	for len(args) >= 2 {
		switch args[0] {
		case "--server":
			c.server = args[1]
		case "--as":
			c.as = args[1]
		case "-o", "--output":
			c.output = args[1]
		default:
			goto parsed
		}
		args = args[2:]
	}
parsed:
	if c.output != "human" && c.output != "json" {
		fmt.Fprintf(os.Stderr, "cloudbox: unknown output %q (want human or json)\n", c.output)
		return exitUsage
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:])
	case "cluster":
		return runCluster(c, args[1:])
	case "app":
		return runApp(c, args[1:])
	case "sandbox":
		return runSandbox(c, args[1:])
	case "apply":
		return runApply(c, args[1:])
	case "test":
		return runTest(c, args[1:])
	case "evidence":
		return runEvidence(c, args[1:])
	case "status":
		return runStatus(c, args[1:])
	case "logs":
		return runLogs(c, args[1:])
	case "exec":
		return runExec(c, args[1:])
	case "port-forward":
		return runPortForward(c, args[1:])
	case "server":
		return runServer(c, args[1:])
	case "version":
		return c.emit((&view{}).add("version", release.Describe()).add("released", release.Released()))
	}
	fmt.Fprintf(os.Stderr, "cloudbox: unknown command %q\n\n%s", args[0], usage)
	return exitUsage
}
