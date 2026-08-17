// The seal on a real cluster (N1–N8, ADR 0001, ADR 0008): standard
// NetworkPolicy v1 whose only admitted egress is cluster DNS and the product
// egress proxy — no vendor policy CRDs, no mesh — verified by a live
// enforcement probe before any sandbox reports itself sealed.
package kube

import (
	"fmt"
	"log"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"cloudbox/internal/cluster"
)

const (
	// proxyLabelKey/Value mark the product egress proxy's pods; the seal
	// admits egress to them and nothing else beyond cluster DNS.
	proxyLabelKey   = "cloudbox.dev/component"
	proxyLabelValue = "egress-proxy"
	proxyName       = "cloudbox-egress-proxy"
	proxyPort       = 3128
	proxyAdminPort  = 3129

	// allowlistConfigMap carries the sealed namespace's FQDN allowlist to the
	// egress proxy (the proxy enforces it; NetworkPolicy v1 cannot).
	allowlistConfigMap = "cloudbox-egress-allowlist"

	// adminTokenSecret carries the per-namespace credential for the proxy's
	// attempt surface. The surface is read-only, but it is the trust root for
	// the violation count in evidence, so it is not an open read either.
	adminTokenSecret = "cloudbox-egress-proxy-token"
	adminTokenKey    = "token"
	// adminTokenHeader is how the control plane presents that credential. It
	// must match cmd/cloudbox-proxy.
	adminTokenHeader = "X-Cloudbox-Egress-Token"

	// canaryImage runs probe canaries; pinned and preloaded into the Kind
	// clusters by hack/conformance/*.sh.
	canaryImage = "busybox:1.36"
)

// SealNamespace installs the default-deny floor as standard NetworkPolicy v1:
// deny all ingress and egress; admit in-sandbox traffic, cluster DNS, and the
// egress proxy (which alone may leave the cluster, enforcing the FQDN
// allowlist). Idempotent: re-sealing updates the policies and allowlist.
func (c *Cluster) SealNamespace(name string, allowlist []string) {
	c.installSealPolicies(name, allowlist)
	// The proxy the policies admit: product-managed, per namespace.
	c.deployEgressProxy(name)
}

// installSealPolicies is the policy-and-allowlist half of the seal; the
// enforcement probe seals its scratch namespaces with this alone, since a
// canary needs the denial semantics, not a proxy deployment.
func (c *Cluster) installSealPolicies(name string, allowlist []string) {
	proxySelector := metav1.LabelSelector{
		MatchLabels: map[string]string{proxyLabelKey: proxyLabelValue},
	}
	dnsPort := intstr.FromInt32(53)
	udp, tcp := corev1.ProtocolUDP, corev1.ProtocolTCP
	proxyTCP := intstr.FromInt32(proxyPort)
	adminTCP := intstr.FromInt32(proxyAdminPort)

	policies := []*networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cloudbox-default-deny", Namespace: name},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress,
				},
			},
		},
		{
			// In-sandbox service-to-service traffic survives the seal.
			ObjectMeta: metav1.ObjectMeta{Name: "cloudbox-allow-in-sandbox", Namespace: name},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress,
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}},
				}},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To: []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cloudbox-allow-cluster-dns", Namespace: name},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To: []networkingv1.NetworkPolicyPeer{{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kube-system"},
						},
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"k8s-app": "kube-dns"},
						},
					}},
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: &udp, Port: &dnsPort},
						{Protocol: &tcp, Port: &dnsPort},
					},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cloudbox-allow-egress-proxy", Namespace: name},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					To:    []networkingv1.NetworkPolicyPeer{{PodSelector: &proxySelector}},
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &tcp, Port: &proxyTCP}},
				}},
			},
		},
		{
			// The proxy alone may leave the cluster; it enforces the FQDN
			// allowlist that NetworkPolicy v1 cannot express (ADR 0001).
			ObjectMeta: metav1.ObjectMeta{Name: "cloudbox-egress-proxy-open", Namespace: name},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: proxySelector,
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
			},
		},
		{
			// The control plane collects the proxy's attempt records through
			// the API server's service proxy, whose traffic is not in-sandbox
			// pod traffic; admit the admin port alone.
			//
			// This rule stays permissive on that port by necessity: the source
			// address of API-server service-proxy traffic is a cluster-topology
			// detail, and encoding it here would make the seal cluster-specific
			// and quietly breakable by a topology change. So the NetworkPolicy
			// is NOT what protects this surface — the per-namespace credential
			// on /attempts is (adminTokenSecret). Stated plainly so nobody
			// later reads this rule as the protection (ADR 0001: residual
			// channels are published, not papered over).
			ObjectMeta: metav1.ObjectMeta{Name: "cloudbox-egress-proxy-admin", Namespace: name},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: proxySelector,
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					Ports: []networkingv1.NetworkPolicyPort{{Protocol: &tcp, Port: &adminTCP}},
				}},
			},
		},
	}
	for _, policy := range policies {
		c.applyNetworkPolicy(policy)
	}

	// The allowlist travels with the seal for the proxy to enforce.
	allow := ""
	for i, fqdn := range allowlist {
		if i > 0 {
			allow += "\n"
		}
		allow += fqdn
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: allowlistConfigMap, Namespace: name},
		Data:       map[string]string{"allowlist": allow},
	}
	cctx, cancel := ctx()
	defer cancel()
	_, err := c.clientset.CoreV1().ConfigMaps(name).Create(cctx, cm, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		_, err = c.clientset.CoreV1().ConfigMaps(name).Update(cctx, cm, metav1.UpdateOptions{})
	}
	if err != nil {
		log.Printf("kube driver: recording allowlist in %q on %s: %v", name, c.name, err)
	}
}

func (c *Cluster) applyNetworkPolicy(policy *networkingv1.NetworkPolicy) {
	cctx, cancel := ctx()
	defer cancel()
	iface := c.clientset.NetworkingV1().NetworkPolicies(policy.Namespace)
	_, err := iface.Create(cctx, policy, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		_, err = iface.Update(cctx, policy, metav1.UpdateOptions{})
	}
	if err != nil {
		log.Printf("kube driver: applying policy %s/%s on %s: %v",
			policy.Namespace, policy.Name, c.name, err)
	}
}

// ProbeEnforcement verifies denial is real (N7): a canary in a freshly sealed
// scratch namespace must FAIL to reach a destination that a canary in an
// unsealed namespace can reach. The baseline leg keeps the probe honest — a
// broken network would fail both legs and read as unproven enforcement, never
// as a pass. A CNI that accepts-but-ignores NetworkPolicy connects on both
// legs and fails the probe.
func (c *Cluster) ProbeEnforcement() bool {
	target, ok := c.probeTarget()
	if !ok {
		return false
	}
	suffix := time.Now().UnixNano()
	baselineNS := fmt.Sprintf("cloudbox-probe-base-%d", suffix)
	sealedNS := fmt.Sprintf("cloudbox-probe-seal-%d", suffix)

	c.EnsureNamespace(baselineNS)
	c.EnsureNamespace(sealedNS)
	defer c.DeleteNamespace(baselineNS)
	defer c.DeleteNamespace(sealedNS)

	c.installSealPolicies(sealedNS, nil)
	if !c.startCanary(baselineNS, target) || !c.startCanary(sealedNS, target) {
		return false
	}

	baselineConnected, baselineOK := c.canaryResult(baselineNS)
	sealedConnected, sealedOK := c.canaryResult(sealedNS)
	if !baselineOK || !sealedOK {
		return false
	}
	if !baselineConnected {
		log.Printf("kube driver: probe baseline could not connect on %s — enforcement unproven", c.name)
		return false
	}
	return !sealedConnected
}

// probeTarget is the kubernetes API service's ClusterIP: always present,
// always accepting connections, and outside any sandbox namespace — exactly
// what a sealed namespace must not reach.
func (c *Cluster) probeTarget() (string, bool) {
	cctx, cancel := ctx()
	defer cancel()
	svc, err := c.clientset.CoreV1().Services("default").Get(cctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		log.Printf("kube driver: resolving probe target on %s: %v", c.name, err)
		return "", false
	}
	return fmt.Sprintf("%s 443", svc.Spec.ClusterIP), true
}

func (c *Cluster) startCanary(namespace, target string) bool {
	deadline := int64(60)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "canary", Namespace: namespace},
		Spec: corev1.PodSpec{
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: &deadline,
			Containers: []corev1.Container{{
				Name:    "canary",
				Image:   canaryImage,
				Command: []string{"sh", "-c", "nc -w 3 " + target + " </dev/null"},
			}},
		},
	}
	cctx, cancel := ctx()
	defer cancel()
	_, err := c.clientset.CoreV1().Pods(namespace).Create(cctx, pod, metav1.CreateOptions{})
	if err != nil {
		log.Printf("kube driver: creating probe canary in %q on %s: %v", namespace, c.name, err)
		return false
	}
	return true
}

// canaryResult waits for the canary to finish and reports whether it
// connected. The second return is false when the canary never completed —
// unproven, which callers must treat as probe failure.
func (c *Cluster) canaryResult(namespace string) (connected, completed bool) {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		cctx, cancel := ctx()
		pod, err := c.clientset.CoreV1().Pods(namespace).Get(cctx, "canary", metav1.GetOptions{})
		cancel()
		if err != nil {
			log.Printf("kube driver: reading probe canary in %q on %s: %v", namespace, c.name, err)
			return false, false
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			return true, true
		case corev1.PodFailed:
			return false, true
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("kube driver: probe canary in %q on %s never completed", namespace, c.name)
	return false, false
}

// AttemptEgress evaluates one connection attempt against what the cluster
// actually enforces for the namespace: whether it carries the seal, what its
// live allowlist says, and what is reachable inside it. It MUST NOT report an
// attempt it did not evaluate as allowed — an unevaluated "allowed,
// unfiltered" is a false containment claim, which is the one thing this driver
// is not allowed to produce.
//
// BOUNDED CLAIM: this evaluates policy, not a packet. It answers what the
// cluster's own configuration admits for this namespace right now; it does not
// send traffic and so cannot catch a CNI that accepted the policies and stopped
// enforcing them. That claim belongs to ProbeEnforcement, which proves denial
// with a live canary before any sandbox reports itself sealed, and to the
// conformance egress scenarios, which assert from a real workload's own output.
// Packet-level enforcement stays the probe's claim; this is the policy's.
func (c *Cluster) AttemptEgress(namespace, destination string) cluster.EgressResult {
	denied := cluster.EgressResult{Allowed: false, Via: "denied"}
	dest := strings.TrimSuffix(strings.TrimSpace(destination), ".")
	if dest == "" {
		return denied
	}
	if !c.namespaceExists(namespace) {
		// No namespace, nothing running in it, nothing to report as allowed.
		return denied
	}
	if !c.namespaceSealed(namespace) {
		// Honest about the absence of a seal rather than reporting filtering
		// that is not in force (N7's silent failure mode is the reason this
		// distinction matters).
		return cluster.EgressResult{Allowed: true, Via: "unfiltered"}
	}
	if c.serviceInNamespace(namespace, dest) {
		return cluster.EgressResult{Allowed: true, Via: "in-sandbox"}
	}
	if c.isClusterDNS(dest) {
		return cluster.EgressResult{Allowed: true, Via: "cluster-dns"}
	}
	if c.onLiveAllowlist(namespace, dest) {
		// The seal admits no direct egress, so an allowlisted FQDN is reachable
		// only through the proxy that enforces the allowlist (ADR 0001).
		return cluster.EgressResult{Allowed: true, Via: "egress-proxy"}
	}
	return denied
}

func (c *Cluster) namespaceExists(name string) bool {
	cctx, cancel := ctx()
	defer cancel()
	_, err := c.clientset.CoreV1().Namespaces().Get(cctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		log.Printf("kube driver: reading namespace %q on %s: %v", name, c.name, err)
	}
	return err == nil
}

// namespaceSealed reports whether the default-deny floor is actually installed
// on the namespace — the seal's presence is read from the cluster, never
// assumed from the control plane's own record.
func (c *Cluster) namespaceSealed(namespace string) bool {
	cctx, cancel := ctx()
	defer cancel()
	_, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).
		Get(cctx, "cloudbox-default-deny", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		log.Printf("kube driver: reading the seal on %q on %s: %v", namespace, c.name, err)
	}
	return err == nil
}

// serviceInNamespace reports whether the destination names a Service of this
// namespace, in any of the forms cluster DNS resolves it by. Cluster DNS
// resolves Services, not workloads (sim DIVERGENCES.md entry 1), so the
// Service object is what makes the destination in-sandbox.
func (c *Cluster) serviceInNamespace(namespace, destination string) bool {
	labels := strings.Split(destination, ".")
	switch len(labels) {
	case 1: // web
	case 2, 3, 5: // web.<ns> | web.<ns>.svc | web.<ns>.svc.cluster.local
		if labels[1] != namespace {
			return false
		}
		if len(labels) >= 3 && labels[2] != "svc" {
			return false
		}
		if len(labels) == 5 && (labels[3] != "cluster" || labels[4] != "local") {
			return false
		}
	default:
		return false
	}
	cctx, cancel := ctx()
	defer cancel()
	_, err := c.clientset.CoreV1().Services(namespace).
		Get(cctx, labels[0], metav1.GetOptions{})
	return err == nil
}

// isClusterDNS reports whether the destination is the cluster's DNS service —
// the one destination outside the namespace the seal admits, by name or by the
// address it resolves to.
func (c *Cluster) isClusterDNS(destination string) bool {
	cctx, cancel := ctx()
	defer cancel()
	svc, err := c.clientset.CoreV1().Services("kube-system").
		Get(cctx, "kube-dns", metav1.GetOptions{})
	if err != nil {
		log.Printf("kube driver: resolving cluster DNS on %s: %v", c.name, err)
		return false
	}
	if svc.Spec.ClusterIP != "" && destination == svc.Spec.ClusterIP {
		return true
	}
	for _, form := range []string{
		svc.Name + "." + svc.Namespace,
		svc.Name + "." + svc.Namespace + ".svc",
		svc.Name + "." + svc.Namespace + ".svc.cluster.local",
	} {
		if destination == form {
			return true
		}
	}
	return false
}

// onLiveAllowlist reads the allowlist the namespace's proxy is actually
// enforcing, not the application record it was sealed from: a re-seal that
// failed to land must not make this report an allowlist that is not in force.
// Exact FQDN match — wildcard and subdomain entries are deliberately not
// allowlist semantics in v1.
func (c *Cluster) onLiveAllowlist(namespace, destination string) bool {
	cctx, cancel := ctx()
	defer cancel()
	cm, err := c.clientset.CoreV1().ConfigMaps(namespace).
		Get(cctx, allowlistConfigMap, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Printf("kube driver: reading the allowlist in %q on %s: %v", namespace, c.name, err)
		}
		return false
	}
	for _, line := range strings.Split(cm.Data["allowlist"], "\n") {
		if fqdn := strings.TrimSpace(line); fqdn != "" && fqdn == destination {
			return true
		}
	}
	return false
}
