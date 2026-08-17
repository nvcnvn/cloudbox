// In-cluster DNS naming, shared by the drivers.
//
// Both the sim and the kube driver have to answer "does this destination name
// something inside the sandbox, or the cluster's DNS?" from a single
// destination string (the frozen contract's AttemptEgress takes nothing else).
// The parsing lives here so the two cannot drift on it: the sim answering
// differently from the real driver about the same name is exactly the class of
// divergence internal/sim/DIVERGENCES.md exists to record.
package cluster

import "strings"

// clusterDNSService is the DNS service every conformant cluster runs.
const clusterDNSService, clusterDNSNamespace = "kube-dns", "kube-system"

// InNamespaceServiceName returns the Service name a destination refers to when
// it names a Service of this namespace, in any of the forms cluster DNS
// resolves: the short name, or it qualified with the namespace (optionally
// `.svc`, optionally `.cluster.local`). The second result is false when the
// destination names something outside the namespace.
//
// Whether such a Service actually exists is the driver's question — the sim
// looks in its own objects, the kube driver asks the API server — because a
// name that resolves to nothing is not in-sandbox (DIVERGENCES entry 1: cluster
// DNS resolves Services, not workloads).
func InNamespaceServiceName(namespace, destination string) (string, bool) {
	dest := strings.TrimSuffix(strings.TrimSpace(destination), ".")
	if dest == "" {
		return "", false
	}
	labels := strings.Split(dest, ".")
	switch len(labels) {
	case 1: // web
		return labels[0], true
	case 2, 3, 5: // web.<ns> | web.<ns>.svc | web.<ns>.svc.cluster.local
		if labels[1] != namespace {
			return "", false
		}
		if len(labels) >= 3 && labels[2] != "svc" {
			return "", false
		}
		if len(labels) == 5 && (labels[3] != "cluster" || labels[4] != "local") {
			return "", false
		}
		return labels[0], true
	default:
		return "", false
	}
}

// IsClusterDNSName reports whether the destination names the cluster's DNS
// service by one of its in-cluster DNS forms. Address-level recognition (the
// service's ClusterIP) needs live cluster state, so it stays with the driver
// that can read it.
func IsClusterDNSName(destination string) bool {
	dest := strings.TrimSuffix(strings.TrimSpace(destination), ".")
	base := clusterDNSService + "." + clusterDNSNamespace
	for _, form := range []string{base, base + ".svc", base + ".svc.cluster.local"} {
		if dest == form {
			return true
		}
	}
	return false
}
