// Workloads on a real cluster: admission applies the manifest as written
// (CP1); readiness and OOM state are observed from the cluster's own status,
// never precomputed (task 3.5, ADR 0008).
package kube

import (
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"cloudbox/internal/cluster"
)

// AddWorkload admits a workload into the namespace by applying its manifest
// to the live cluster. The Ready/OOMKilled fields the control plane
// precomputed for the sim are ignored: on a real cluster those are observed,
// not asserted.
func (c *Cluster) AddWorkload(namespace string, w cluster.Workload) {
	obj := cluster.FromManifest(w.Manifest)
	obj.Namespace = namespace
	if meta, ok := w.Manifest["metadata"].(map[string]any); ok {
		meta["namespace"] = namespace
	}
	c.ApplyRaw(obj)
}

// Workloads reports what actually runs in the namespace, with readiness and
// OOM state read from live status. Deployments are the workload unit the
// product admits; their pods supply OOM evidence.
func (c *Cluster) Workloads(namespace string) []cluster.Workload {
	cctx, cancel := ctx()
	defer cancel()

	var out []cluster.Workload

	deployments, err := c.clientset.AppsV1().Deployments(namespace).List(cctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("kube driver: listing deployments in %q on %s: %v", namespace, c.name, err)
		return nil
	}
	pods, err := c.clientset.CoreV1().Pods(namespace).List(cctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("kube driver: listing pods in %q on %s: %v", namespace, c.name, err)
		pods = &corev1.PodList{}
	}

	oomKilled := map[string]bool{}
	for _, pod := range pods.Items {
		for _, status := range pod.Status.ContainerStatuses {
			terminated := status.LastTerminationState.Terminated
			if terminated == nil {
				terminated = status.State.Terminated
			}
			if terminated != nil && terminated.Reason == "OOMKilled" {
				// Attribute the kill to the owning workload by the pod's
				// app label, falling back to the pod name.
				owner := pod.Labels["app"]
				if owner == "" {
					owner = pod.Name
				}
				oomKilled[owner] = true
			}
		}
	}

	for _, d := range deployments.Items {
		replicas := int32(1)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		out = append(out, cluster.Workload{
			Name:      d.Name,
			Ready:     d.Status.ReadyReplicas >= replicas,
			OOMKilled: oomKilled[d.Name],
		})
	}

	// Bare pods admitted directly (not owned by a deployment).
	for _, pod := range pods.Items {
		if len(pod.OwnerReferences) > 0 {
			continue
		}
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				ready = true
			}
		}
		out = append(out, cluster.Workload{
			Name:      pod.Name,
			Ready:     ready,
			OOMKilled: oomKilled[pod.Name] || oomKilled[pod.Labels["app"]],
		})
	}
	return out
}
