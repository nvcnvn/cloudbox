// Package kube is the real-Kubernetes cluster driver (ADR 0008): the
// production path behind the frozen cluster contract that the sim driver
// models. Every method implements the contract as written — the interface
// gains nothing for this driver's convenience; where real Kubernetes cannot
// satisfy a method, that is a spec-visible finding, not a signature change.
//
// A Driver is a fleet of clusters named by kubeconfig context: Cluster(name)
// serves the context of that name from the kubeconfig resolved by the
// standard loading rules (KUBECONFIG or ~/.kube/config). Clients are built
// lazily and cached per context.
//
// The contract's methods return no errors (it is shaped by what the control
// plane needs, not by transport concerns), so this driver degrades honestly
// instead of loudly: a failed call is logged, and the observable state simply
// does not show what failed to happen — an uninstalled CRD is absent from
// ListCRDs, an unsealed namespace fails the enforcement probe. The honest
// failure is load-bearing: a sandbox is never reported sealed on a cluster
// this driver could not actually seal.
package kube

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"cloudbox/internal/cluster"
)

// The frozen contract is satisfied as written (ADR 0008).
var (
	_ cluster.Driver  = (*Driver)(nil)
	_ cluster.Cluster = (*Cluster)(nil)
)

// callTimeout bounds every API round trip; the contract has no contexts.
const callTimeout = 30 * time.Second

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), callTimeout)
}

// Driver implements cluster.Driver over kubeconfig contexts.
type Driver struct {
	mu       sync.Mutex
	raw      clientcmdapi.Config
	clusters map[string]*Cluster
}

// NewDriver loads the kubeconfig via the standard loading rules. A missing or
// unreadable kubeconfig is a boot-time error: a kube-driver control plane
// with no cluster access is misconfiguration, not a state to run in.
func NewDriver() (*Driver, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	raw, err := rules.Load()
	if err != nil {
		return nil, err
	}
	return &Driver{raw: *raw, clusters: map[string]*Cluster{}}, nil
}

// Cluster returns the cluster behind the kubeconfig context of this name.
func (d *Driver) Cluster(name string) (cluster.Cluster, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if c, ok := d.clusters[name]; ok {
		return c, true
	}
	if _, ok := d.raw.Contexts[name]; !ok {
		return nil, false
	}
	cfg, err := clientcmd.NewNonInteractiveClientConfig(
		d.raw, name, &clientcmd.ConfigOverrides{}, nil,
	).ClientConfig()
	if err != nil {
		log.Printf("kube driver: building client config for context %q: %v", name, err)
		return nil, false
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Printf("kube driver: clientset for context %q: %v", name, err)
		return nil, false
	}
	extensions, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		log.Printf("kube driver: apiextensions client for context %q: %v", name, err)
		return nil, false
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Printf("kube driver: dynamic client for context %q: %v", name, err)
		return nil, false
	}
	c := &Cluster{
		name:        name,
		clientset:   clientset,
		extensions:  extensions,
		dynamic:     dyn,
		discovery:   clientset.Discovery(),
		adminTokens: map[string]string{},
	}
	d.clusters[name] = c
	return c, true
}

// Cluster is one real cluster's control surface.
type Cluster struct {
	name       string
	clientset  kubernetes.Interface
	extensions apiextensionsclient.Interface
	dynamic    dynamic.Interface
	discovery  discovery.DiscoveryInterface

	// adminTokens caches each sealed namespace's egress-proxy credential; it
	// is fixed for the namespace's life (see proxy.go).
	tokenMu     sync.Mutex
	adminTokens map[string]string
}

func (c *Cluster) Name() string { return c.name }

// UserControlled reports false: a cluster reached through the control plane's
// own kubeconfig is in control-plane custody. User-controlled (developer
// local) clusters are the sim-modelled S3 path; provisioning them is not a
// kube-driver capability in this change.
func (c *Cluster) UserControlled() bool { return false }

// InstallCRDs creates each product CRD on the real cluster; already-present
// definitions are left as they are.
func (c *Cluster) InstallCRDs(crds []cluster.CRD) {
	for _, crd := range crds {
		plural := strings.TrimSuffix(crd.Name, "."+crd.Group)
		preserve := true
		obj := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: crd.Name},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: crd.Group,
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   plural,
					Singular: strings.ToLower(crd.Kind),
					Kind:     crd.Kind,
					ListKind: crd.Kind + "List",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: &preserve,
						},
					},
				}},
			},
		}
		cctx, cancel := ctx()
		_, err := c.extensions.ApiextensionsV1().CustomResourceDefinitions().
			Create(cctx, obj, metav1.CreateOptions{})
		cancel()
		if err != nil && !apierrors.IsAlreadyExists(err) {
			log.Printf("kube driver: installing CRD %s on %s: %v", crd.Name, c.name, err)
		}
	}
}

// ListCRDs reads the definitions actually present on the live cluster.
func (c *Cluster) ListCRDs() []cluster.CRD {
	cctx, cancel := ctx()
	defer cancel()
	list, err := c.extensions.ApiextensionsV1().CustomResourceDefinitions().
		List(cctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("kube driver: listing CRDs on %s: %v", c.name, err)
		return nil
	}
	crds := make([]cluster.CRD, 0, len(list.Items))
	for _, item := range list.Items {
		crds = append(crds, cluster.CRD{
			Name:  item.Name,
			Group: item.Spec.Group,
			Kind:  item.Spec.Names.Kind,
		})
	}
	return crds
}

// restMapping resolves apiVersion/kind to a resource. Group comes from the
// object's own apiVersion when present; a bare kind is searched across the
// live API surface.
func (c *Cluster) restMapping(apiVersion, kind string) (*schema.GroupVersionResource, bool, error) {
	groups, err := restmapper.GetAPIGroupResources(c.discovery)
	if err != nil {
		return nil, false, err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groups)
	gv := schema.GroupVersion{}
	if apiVersion != "" {
		parsed, err := schema.ParseGroupVersion(apiVersion)
		if err == nil {
			gv = parsed
		}
	}
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: kind}, gv.Version)
	if err != nil {
		// Bare kind (GetRaw has no apiVersion): search all groups.
		mapping, err = mapper.RESTMapping(schema.GroupKind{Kind: kind})
		if err != nil {
			return nil, false, err
		}
	}
	namespaced := mapping.Scope.Name() == "namespace"
	return &mapping.Resource, namespaced, nil
}

// ApplyRaw creates or replaces the object as the user wrote it (CP1: the
// manifest travels whole).
func (c *Cluster) ApplyRaw(obj cluster.Object) {
	gvr, namespaced, err := c.restMapping(obj.APIVersion, obj.Kind)
	if err != nil {
		log.Printf("kube driver: mapping %s/%s on %s: %v", obj.APIVersion, obj.Kind, c.name, err)
		return
	}
	u := &unstructured.Unstructured{Object: obj.Manifest}
	var iface dynamic.ResourceInterface = c.dynamic.Resource(*gvr)
	if namespaced {
		ns := obj.Namespace
		if ns == "" {
			ns = "default"
		}
		iface = c.dynamic.Resource(*gvr).Namespace(ns)
	}
	cctx, cancel := ctx()
	defer cancel()
	_, err = iface.Create(cctx, u, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, getErr := iface.Get(cctx, obj.Name, metav1.GetOptions{})
		if getErr != nil {
			log.Printf("kube driver: refetching %s %q on %s: %v", obj.Kind, obj.Name, c.name, getErr)
			return
		}
		u.SetResourceVersion(existing.GetResourceVersion())
		_, err = iface.Update(cctx, u, metav1.UpdateOptions{})
	}
	if err != nil {
		log.Printf("kube driver: applying %s %q on %s: %v", obj.Kind, obj.Name, c.name, err)
	}
}

// GetRaw reads one object back from the live cluster.
func (c *Cluster) GetRaw(namespace, kind, name string) (cluster.Object, bool) {
	gvr, namespaced, err := c.restMapping("", kind)
	if err != nil {
		log.Printf("kube driver: mapping kind %q on %s: %v", kind, c.name, err)
		return cluster.Object{}, false
	}
	var iface dynamic.ResourceInterface = c.dynamic.Resource(*gvr)
	if namespaced {
		if namespace == "" {
			namespace = "default"
		}
		iface = c.dynamic.Resource(*gvr).Namespace(namespace)
	}
	cctx, cancel := ctx()
	defer cancel()
	u, err := iface.Get(cctx, name, metav1.GetOptions{})
	if err != nil {
		return cluster.Object{}, false
	}
	return cluster.FromManifest(u.Object), true
}

// KubernetesMinor reads the server version from the live cluster (ADR 0006:
// substrate digests reflect what is actually installed).
func (c *Cluster) KubernetesMinor() string {
	version, err := c.discovery.ServerVersion()
	if err != nil {
		log.Printf("kube driver: server version on %s: %v", c.name, err)
		return ""
	}
	minor := strings.TrimSuffix(version.Minor, "+")
	return version.Major + "." + minor
}

// SetComponents is sim-only arrangement: on a real cluster the installed
// substrate is not settable through the control plane — Components reads the
// live state instead (ADR 0006). Nothing reaches this under the kube driver
// (the /simctl surface that exposes it is never registered); it is a no-op
// rather than a lie.
func (c *Cluster) SetComponents(k8sMinor string, comps []cluster.Component) {}

// EnsureNamespace creates the namespace if it is absent.
func (c *Cluster) EnsureNamespace(name string) {
	cctx, cancel := ctx()
	defer cancel()
	_, err := c.clientset.CoreV1().Namespaces().Create(cctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		log.Printf("kube driver: creating namespace %q on %s: %v", name, c.name, err)
	}
}

// DeleteNamespace removes the namespace and everything in it.
func (c *Cluster) DeleteNamespace(name string) {
	cctx, cancel := ctx()
	defer cancel()
	err := c.clientset.CoreV1().Namespaces().Delete(cctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		log.Printf("kube driver: deleting namespace %q on %s: %v", name, c.name, err)
	}
}
