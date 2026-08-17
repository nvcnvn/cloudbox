// The product egress proxy's deployment shape under the kube driver: one
// proxy per sealed namespace, deployed at seal time, reached by workloads via
// the ADR 0001 env-var fallback (http_proxy injected at admission). The
// control plane collects its attempt records through the API server's
// service proxy — no direct network path is assumed between cloudboxd and
// the cluster's pod network.
package kube

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"cloudbox/internal/cluster"
)

// proxyImage is provisioned into the conformance clusters by
// hack/conformance/*.sh (built from cmd/cloudbox-proxy).
func proxyImage() string {
	if img := os.Getenv("CLOUDBOX_PROXY_IMAGE"); img != "" {
		return img
	}
	return "cloudbox-egress-proxy:dev"
}

// ensureAdminToken creates the namespace's admin credential if it is absent
// and returns it. An existing token is kept as it is: re-sealing (an allowlist
// change) must not rotate a credential the running proxy has already mounted
// and the collector may be mid-read with.
func (c *Cluster) ensureAdminToken(namespace string) (string, bool) {
	cctx, cancel := ctx()
	defer cancel()
	secrets := c.clientset.CoreV1().Secrets(namespace)
	if existing, err := secrets.Get(cctx, adminTokenSecret, metav1.GetOptions{}); err == nil {
		if token := string(existing.Data[adminTokenKey]); token != "" {
			return token, true
		}
	} else if !apierrors.IsNotFound(err) {
		log.Printf("kube driver: reading the egress proxy admin token in %q on %s: %v",
			namespace, c.name, err)
		return "", false
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		log.Printf("kube driver: generating the egress proxy admin token for %q: %v", namespace, err)
		return "", false
	}
	token := hex.EncodeToString(buf)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: adminTokenSecret, Namespace: namespace},
		Data:       map[string][]byte{adminTokenKey: []byte(token)},
	}
	if _, err := secrets.Create(cctx, secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Printf("kube driver: creating the egress proxy admin token in %q on %s: %v",
				namespace, c.name, err)
			return "", false
		}
		// Created concurrently; the stored one is authoritative.
		existing, err := secrets.Get(cctx, adminTokenSecret, metav1.GetOptions{})
		if err != nil {
			return "", false
		}
		return string(existing.Data[adminTokenKey]), true
	}
	return token, true
}

// adminToken returns the namespace's admin credential, cached: it is fixed for
// the namespace's life, and collection runs on a timer, so re-reading the
// Secret every round would double the API traffic for a value that cannot have
// changed. A failed read drops the cached entry so the next round re-reads.
func (c *Cluster) adminToken(namespace string) (string, bool) {
	c.tokenMu.Lock()
	cached, ok := c.adminTokens[namespace]
	c.tokenMu.Unlock()
	if ok {
		return cached, true
	}
	cctx, cancel := ctx()
	defer cancel()
	secret, err := c.clientset.CoreV1().Secrets(namespace).
		Get(cctx, adminTokenSecret, metav1.GetOptions{})
	if err != nil {
		log.Printf("kube driver: reading the egress proxy admin token in %q on %s: %v",
			namespace, c.name, err)
		return "", false
	}
	token := string(secret.Data[adminTokenKey])
	if token == "" {
		return "", false
	}
	c.tokenMu.Lock()
	c.adminTokens[namespace] = token
	c.tokenMu.Unlock()
	return token, true
}

func (c *Cluster) forgetAdminToken(namespace string) {
	c.tokenMu.Lock()
	delete(c.adminTokens, namespace)
	c.tokenMu.Unlock()
}

// deployEgressProxy installs (or updates) the namespace's egress proxy and
// its service. Part of sealing: the seal admits egress only to this proxy.
func (c *Cluster) deployEgressProxy(namespace string) {
	// The credential must exist before the pod that mounts it.
	c.ensureAdminToken(namespace)
	labels := map[string]string{proxyLabelKey: proxyLabelValue}
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: namespace, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "proxy",
						Image:           proxyImage(),
						ImagePullPolicy: corev1.PullIfNotPresent,
						VolumeMounts: []corev1.VolumeMount{
							{Name: "allowlist", MountPath: "/etc/cloudbox"},
							{Name: "admin-token", MountPath: "/etc/cloudbox/admin", ReadOnly: true},
						},
						// A restart costs records and marks the sandbox's
						// egress record incomplete, so the proxy gets what it
						// needs not to be evicted or OOM-killed in the
						// ordinary course of a run: a request the scheduler
						// must honour, a limit its bounded record stays far
						// inside, and a readiness gate so a replacement is not
						// treated as serving before it can answer.
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("25m"),
								corev1.ResourceMemory: resource.MustParse("32Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt32(proxyAdminPort),
								},
							},
							InitialDelaySeconds: 1,
							PeriodSeconds:       5,
							TimeoutSeconds:      2,
							FailureThreshold:    3,
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "allowlist",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: allowlistConfigMap,
									},
								},
							},
						},
						{
							Name: "admin-token",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: adminTokenSecret,
								},
							},
						},
					},
				},
			},
		},
	}
	cctx, cancel := ctx()
	defer cancel()
	deployments := c.clientset.AppsV1().Deployments(namespace)
	if _, err := deployments.Create(cctx, deployment, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			_, err = deployments.Update(cctx, deployment, metav1.UpdateOptions{})
		}
		if err != nil && !apierrors.IsAlreadyExists(err) {
			log.Printf("kube driver: deploying egress proxy in %q on %s: %v", namespace, c.name, err)
		}
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: namespace, Labels: labels},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{Name: "proxy", Port: proxyPort, TargetPort: intstr.FromInt32(proxyPort)},
				{Name: "admin", Port: proxyAdminPort, TargetPort: intstr.FromInt32(proxyAdminPort)},
			},
		},
	}
	if _, err := c.clientset.CoreV1().Services(namespace).Create(cctx, service, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		log.Printf("kube driver: creating egress proxy service in %q on %s: %v", namespace, c.name, err)
	}
}

// proxyAttempt is one record as the proxy's admin surface serves it.
type proxyAttempt struct {
	Destination string      `json:"destination"`
	SourceIP    string      `json:"sourceIp"`
	At          metav1.Time `json:"at"`
	Allowed     bool        `json:"allowed"`
}

// proxyAttemptRecord is the /attempts response: the retained attempts, what
// the proxy's bound discarded, and which proxy process answered.
type proxyAttemptRecord struct {
	Attempts    []proxyAttempt `json:"attempts"`
	Dropped     int            `json:"dropped"`
	Incarnation string         `json:"incarnation"`
}

// decodeAttemptRecord accepts both shapes the admin surface can serve. A
// namespace sealed before this change runs an older proxy image that answers
// with a bare array, and its proxy is only replaced by a re-seal; the
// collector keeps reporting for it rather than logging a decode failure and
// silently recording nothing. An older proxy has no drop counter, so what it
// reports is all it can account for.
func decodeAttemptRecord(raw []byte) (proxyAttemptRecord, error) {
	var record proxyAttemptRecord
	objectErr := json.Unmarshal(raw, &record)
	if objectErr == nil {
		return record, nil
	}
	var preChange []proxyAttempt
	if err := json.Unmarshal(raw, &preChange); err == nil {
		return proxyAttemptRecord{Attempts: preChange}, nil
	}
	return proxyAttemptRecord{}, objectErr
}

// EgressAttempts implements the optional cluster.EgressObserver capability:
// the proxy's records, attributed to workloads by resolving each recorded
// source IP against the namespace's live pods, together with what the proxy's
// retention bound discarded.
func (c *Cluster) EgressAttempts(namespace string) cluster.EgressReport {
	token, ok := c.adminToken(namespace)
	if !ok {
		return cluster.EgressReport{}
	}
	cctx, cancel := ctx()
	defer cancel()
	// Through the API server's service proxy, as before, but presenting the
	// namespace's credential: the attempt surface is the trust root for the
	// violation count in evidence, so it is not an open read on the cluster.
	// The header survives the service proxy; the credential stays out of the
	// request URI the API server audits.
	raw, err := c.clientset.CoreV1().RESTClient().Get().
		Namespace(namespace).
		Resource("services").
		Name("http:" + proxyName + ":admin").
		SubResource("proxy").
		Suffix("attempts").
		SetHeader(adminTokenHeader, token).
		DoRaw(cctx)
	if err != nil {
		log.Printf("kube driver: reading egress attempts in %q on %s: %v", namespace, c.name, err)
		// The credential may have been the problem; re-read it next round
		// rather than failing forever on a cached value.
		c.forgetAdminToken(namespace)
		return cluster.EgressReport{}
	}
	record, err := decodeAttemptRecord(raw)
	if err != nil {
		log.Printf("kube driver: decoding egress attempts in %q on %s: %v", namespace, c.name, err)
		return cluster.EgressReport{}
	}
	records := record.Attempts

	// Attribute source IPs to workloads via the namespace's live pods.
	byIP := map[string]string{}
	if pods, err := c.clientset.CoreV1().Pods(namespace).List(cctx, metav1.ListOptions{}); err == nil {
		for _, pod := range pods.Items {
			owner := pod.Labels["app"]
			if owner == "" {
				owner = pod.Name
			}
			if pod.Status.PodIP != "" {
				byIP[pod.Status.PodIP] = owner
			}
		}
	}

	attempts := make([]cluster.EgressAttempt, 0, len(records))
	for _, r := range records {
		workload := byIP[r.SourceIP]
		if workload == "" {
			workload = r.SourceIP
		}
		attempts = append(attempts, cluster.EgressAttempt{
			Destination: r.Destination,
			Workload:    workload,
			At:          r.At.Time,
			Allowed:     r.Allowed,
		})
	}
	return cluster.EgressReport{
		Attempts:    attempts,
		Dropped:     record.Dropped,
		Incarnation: record.Incarnation,
		Collected:   true,
	}
}

// The optional capability is actually satisfied.
var _ cluster.EgressObserver = (*Cluster)(nil)
