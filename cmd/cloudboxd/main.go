// cloudboxd is the CloudBox control plane: all validation, bundling, evidence
// gathering, signing, and enforcement run here, server-side (ADR 0004).
package main

import (
	"flag"
	"log"
	"net/http"

	"cloudbox/internal/server"
	"cloudbox/internal/sim"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	driver := flag.String("driver", "sim", "cluster driver: sim (simulated clusters) or kube")
	flag.Parse()

	switch *driver {
	case "sim":
		srv := server.New(sim.NewWorld())
		log.Printf("cloudboxd listening on %s (driver=sim)", *addr)
		log.Fatal(http.ListenAndServe(*addr, srv))
	default:
		// The kube driver is the production path (ADR 0007); it lands behind
		// the same server API once the sim-verified behavior contracts hold.
		log.Fatalf("cloudboxd: unsupported driver %q", *driver)
	}
}
