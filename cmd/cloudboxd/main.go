// cloudboxd is the CloudBox control plane: all validation, bundling, evidence
// gathering, signing, and enforcement run here, server-side (ADR 0004).
package main

import (
	"flag"
	"log"
	"net/http"

	"cloudbox/internal/cluster/kube"
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
	case "kube":
		// The production path (ADR 0008): real clusters named by kubeconfig
		// context, real clock, and no /simctl arrangement surface.
		d, err := kube.NewDriver()
		if err != nil {
			log.Fatalf("cloudboxd: loading kubeconfig for the kube driver: %v", err)
		}
		srv := server.NewWithDriver(d)
		log.Printf("cloudboxd listening on %s (driver=kube)", *addr)
		log.Fatal(http.ListenAndServe(*addr, srv))
	default:
		log.Fatalf("cloudboxd: unsupported driver %q", *driver)
	}
}
