// egress-eval asks the kube driver how it evaluates one connection attempt.
// It is conformance test infrastructure, not product surface.
//
// The cluster contract's AttemptEgress has no caller under --driver kube: its
// only product exposure is the /simctl arrangement surface, which exists solely
// when the sim driver is constructed (ADR 0008), and nothing in the product
// routes to it. The conformance scenarios for "Egress evaluation reflects live
// cluster state" therefore drive the contract method directly, rather than the
// product growing an endpoint it does not otherwise need — and rather than a
// test-arrangement route becoming reachable under the kube driver, which the
// same ADR forbids.
//
// What it evaluates is the live cluster's policy for a namespace, so the
// scenario still arranges its sandbox through the product API; this only asks
// the question and prints the answer.
//
//	egress-eval -context kind-cloudbox-conformance -namespace app-x-sbx-1 \
//	    -destination example.com
//	{"allowed":true,"via":"egress-proxy"}
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"cloudbox/internal/cluster/kube"
)

func main() {
	contextName := flag.String("context", "", "kubeconfig context of the cluster to ask")
	namespace := flag.String("namespace", "", "namespace the attempt comes from")
	destination := flag.String("destination", "", "destination attempted")
	flag.Parse()

	if *contextName == "" || *namespace == "" || *destination == "" {
		log.Fatal("egress-eval: -context, -namespace and -destination are all required")
	}

	driver, err := kube.NewDriver()
	if err != nil {
		log.Fatalf("egress-eval: loading kubeconfig: %v", err)
	}
	cl, ok := driver.Cluster(*contextName)
	if !ok {
		log.Fatalf("egress-eval: no cluster for kubeconfig context %q", *contextName)
	}
	if err := json.NewEncoder(os.Stdout).Encode(cl.AttemptEgress(*namespace, *destination)); err != nil {
		log.Fatalf("egress-eval: writing the result: %v", err)
	}
}
