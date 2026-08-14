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
	switch os.Args[1] {
	case "version":
		fmt.Println("cloudbox v1 (development)")
	default:
		fmt.Fprintf(os.Stderr, "cloudbox: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}
