#!/usr/bin/env bash
# Provision the enforcing conformance cluster: Kind + Calico (ADR 0008).
#
# Calico is installed explicitly as the enforcing CNI. kind v0.32.0's default
# CNI was VERIFIED to enforce NetworkPolicy (see README.md), but the
# conformance claim never rides on a default that the next version bump could
# change; the enforcing CNI is always pinned and explicit.
#
# Idempotent: re-running creates only what is missing and always re-exports
# the kubeconfig.
set -euo pipefail

CLUSTER=cloudbox-conformance
KIND_PIN=v0.32.0
NODE_IMAGE="kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5"
CALICO_MANIFEST="https://raw.githubusercontent.com/projectcalico/calico/v3.31.0/manifests/calico.yaml"
CANARY_IMAGE="busybox:1.36"  # the enforcement-probe canary (internal/cluster/kube/seal.go)

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
KUBECONFIG_OUT="$REPO/acceptance-tests/.kube/conformance.kubeconfig"

installed="$(kind version -q 2>/dev/null || kind version | awk '{print $2}')"
if [ "${installed#v}" != "${KIND_PIN#v}" ]; then
  echo "WARNING: kind $installed differs from the pinned $KIND_PIN; the node" >&2
  echo "image is digest-pinned so the cluster is still reproducible, but" >&2
  echo "re-verify before changing the pin (see README.md)." >&2
fi

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  # No --wait: nodes stay NotReady until the CNI below is installed.
  kind create cluster --name "$CLUSTER" --image "$NODE_IMAGE" --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true    # Calico below is the enforcing CNI
  podSubnet: 192.168.0.0/16  # calico.yaml's default pool
EOF
fi

mkdir -p "$(dirname "$KUBECONFIG_OUT")"
kind get kubeconfig --name "$CLUSTER" > "$KUBECONFIG_OUT"
export KUBECONFIG="$KUBECONFIG_OUT"

# Preload the probe canary image so enforcement probes never wait on a
# registry pull mid-probe. Pulled inside the node with crictl: `kind load`
# cannot import archives from Docker Desktop's containerd image store.
for node in $(kind get nodes --name "$CLUSTER"); do
  docker exec "$node" crictl pull "docker.io/library/$CANARY_IMAGE"
done

if ! kubectl get daemonset calico-node -n kube-system >/dev/null 2>&1; then
  kubectl apply -f "$CALICO_MANIFEST"
fi
kubectl -n kube-system rollout status daemonset/calico-node --timeout=300s
kubectl wait --for=condition=Ready nodes --all --timeout=300s

echo "enforcing conformance cluster '$CLUSTER' ready"
echo "kubeconfig: $KUBECONFIG_OUT"
