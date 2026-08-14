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
	// uses it — it must work with no cluster and no server at all.
	if len(args) >= 2 && args[0] == "--server" {
		args = args[2:]
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch args[0] {
	case "check":
		os.Exit(runCheck(args[1:]))
	case "version":
		fmt.Println("cloudbox v1 (development)")
	default:
		fmt.Fprintf(os.Stderr, "cloudbox: unknown command %q\n\n%s", args[0], usage)
		os.Exit(2)
	}
}
