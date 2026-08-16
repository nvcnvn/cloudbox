// Substrate reads from the live cluster (P1–P4, ADR 0006): what a substrate
// digest is computed from is what is actually installed, never a declaration.
//
// The component model maps onto real cluster state:
//
//   - "operator": a Deployment labeled app.kubernetes.io/component=operator
//     (any namespace). Its version is its app.kubernetes.io/version label and
//     the CRD API groups it owns come from the cloudbox.dev/owns-crds
//     annotation (comma-separated); absent that, from installed CRD groups
//     whose names contain the deployment's app.kubernetes.io/name.
//   - "admission": each Validating/MutatingWebhookConfiguration.
//   - "class": StorageClasses, IngressClasses and PriorityClasses present on
//     the cluster, grouped per kind.
package kube

import (
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"cloudbox/internal/cluster"
)

// Components reads the installed substrate from the live cluster.
func (c *Cluster) Components() []cluster.Component {
	cctx, cancel := ctx()
	defer cancel()

	var out []cluster.Component

	// Operators: labeled deployments across all namespaces.
	deployments, err := c.clientset.AppsV1().Deployments("").List(cctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=operator",
	})
	if err != nil {
		log.Printf("kube driver: listing operator deployments on %s: %v", c.name, err)
	} else {
		crdGroups := map[string]bool{}
		for _, crd := range c.ListCRDs() {
			crdGroups[crd.Group] = true
		}
		for _, d := range deployments.Items {
			name := d.Labels["app.kubernetes.io/name"]
			if name == "" {
				name = d.Name
			}
			comp := cluster.Component{
				Name:    name,
				Version: d.Labels["app.kubernetes.io/version"],
				Kind:    "operator",
			}
			if owns := d.Annotations["cloudbox.dev/owns-crds"]; owns != "" {
				for _, group := range strings.Split(owns, ",") {
					comp.OwnsCRDs = append(comp.OwnsCRDs, strings.TrimSpace(group))
				}
			} else {
				for group := range crdGroups {
					if strings.Contains(group, name) {
						comp.OwnsCRDs = append(comp.OwnsCRDs, group)
					}
				}
			}
			out = append(out, comp)
		}
	}

	// Admission configurations.
	validating, err := c.clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().
		List(cctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("kube driver: listing validating webhooks on %s: %v", c.name, err)
	} else {
		for _, wh := range validating.Items {
			out = append(out, cluster.Component{Name: wh.Name, Version: "v1", Kind: "admission"})
		}
	}
	mutating, err := c.clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().
		List(cctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("kube driver: listing mutating webhooks on %s: %v", c.name, err)
	} else {
		for _, wh := range mutating.Items {
			out = append(out, cluster.Component{Name: wh.Name, Version: "v1", Kind: "admission"})
		}
	}

	// Named classes.
	if classes := c.classNames(); len(classes) > 0 {
		out = append(out, cluster.Component{
			Name: "cluster-classes", Version: "v1", Kind: "class", Classes: classes,
		})
	}
	return out
}

func (c *Cluster) classNames() []string {
	cctx, cancel := ctx()
	defer cancel()
	var classes []string
	storage, err := c.clientset.StorageV1().StorageClasses().List(cctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("kube driver: listing storage classes on %s: %v", c.name, err)
	} else {
		for _, sc := range storage.Items {
			classes = append(classes, sc.Name)
		}
	}
	ingress, err := c.clientset.NetworkingV1().IngressClasses().List(cctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("kube driver: listing ingress classes on %s: %v", c.name, err)
	} else {
		for _, ic := range ingress.Items {
			classes = append(classes, ic.Name)
		}
	}
	priority, err := c.clientset.SchedulingV1().PriorityClasses().List(cctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("kube driver: listing priority classes on %s: %v", c.name, err)
	} else {
		for _, pc := range priority.Items {
			classes = append(classes, pc.Name)
		}
	}
	return classes
}
