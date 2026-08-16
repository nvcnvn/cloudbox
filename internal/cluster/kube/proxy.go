// The product egress proxy's deployment shape under the kube driver: one
// proxy per sealed namespace, deployed at seal time, reached by workloads via
// the ADR 0001 env-var fallback (http_proxy injected at admission). The
// control plane collects its attempt records through the API server's
// service proxy — no direct network path is assumed between cloudboxd and
// the cluster's pod network.
package kube

import (
	"encoding/json"
	"log"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// deployEgressProxy installs (or updates) the namespace's egress proxy and
// its service. Part of sealing: the seal admits egress only to this proxy.
func (c *Cluster) deployEgressProxy(namespace string) {
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
						VolumeMounts: []corev1.VolumeMount{{
							Name: "allowlist", MountPath: "/etc/cloudbox",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "allowlist",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: allowlistConfigMap,
								},
							},
						},
					}},
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

// EgressAttempts implements the optional cluster.EgressObserver capability:
// the proxy's records, attributed to workloads by resolving each recorded
// source IP against the namespace's live pods.
func (c *Cluster) EgressAttempts(namespace string) []cluster.EgressAttempt {
	cctx, cancel := ctx()
	defer cancel()
	raw, err := c.clientset.CoreV1().Services(namespace).
		ProxyGet("http", proxyName, "admin", "/attempts", nil).DoRaw(cctx)
	if err != nil {
		log.Printf("kube driver: reading egress attempts in %q on %s: %v", namespace, c.name, err)
		return nil
	}
	var records []struct {
		Destination string `json:"destination"`
		SourceIP    string `json:"sourceIp"`
		At          metav1.Time `json:"at"`
		Allowed     bool   `json:"allowed"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		log.Printf("kube driver: decoding egress attempts in %q on %s: %v", namespace, c.name, err)
		return nil
	}

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
	return attempts
}

// The optional capability is actually satisfied.
var _ cluster.EgressObserver = (*Cluster)(nil)
