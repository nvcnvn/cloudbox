// cloudbox is the thin CLI client (ADR 0004): it creates resources and watches
// status via the control plane; the one offline exception is `check` (X3).
package main

import (
	"fmt"
	"os"
)

const usage = `cloudbox — sealed, production-shaped sandboxes with signed evidence

Usage:
  cloudbox <command> [flags]

Commands are added as their capability lands; run a command for its own help.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	args := os.Args[1:]
	// --server names the control plane for online commands; check (X3) never
	// uses it — it must work with no cluster and no server at all. --as is
	// the acting identity (a real deployment gets this from login).
	cl := &client{}
	for len(args) >= 2 {
		if args[0] == "--server" {
			cl.server = args[1]
			args = args[2:]
			continue
		}
		if args[0] == "--as" {
			cl.as = args[1]
			args = args[2:]
			continue
		}
		break
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch args[0] {
	case "check":
		os.Exit(runCheck(args[1:]))
	case "status":
		os.Exit(runStatus(cl, args[1:]))
	case "logs":
		os.Exit(runLogs(cl, args[1:]))
	case "exec":
		os.Exit(runExec(cl, args[1:]))
	case "port-forward":
		os.Exit(runPortForward(cl, args[1:]))
	case "version":
		fmt.Println("cloudbox v1 (development)")
	default:
		fmt.Fprintf(os.Stderr, "cloudbox: unknown command %q\n\n%s", args[0], usage)
		os.Exit(2)
	}
}
