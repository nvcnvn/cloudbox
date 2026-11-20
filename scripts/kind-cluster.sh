#!/usr/bin/env bash
# Stand up a local Kubernetes cluster CloudBox can use as a sandbox host:
# Kind with Calico, a CNI that enforces NetworkPolicy.
#
# This script is a supported convenience that lives OUTSIDE the product
# boundary (ADR 0010): CloudBox never creates clusters. It makes a cluster
# that will pass registration's enforcement probe. You then register it
# yourself as a user-controlled sandbox host.
#
# Usage: scripts/kind-cluster.sh [cluster-name]
#   cluster-name       default: cloudbox
#   KUBECONFIG_OUT     write the cluster's kubeconfig to this file instead
#                      of merging it into your default kubeconfig
#
# Requires docker, kind and kubectl (and Go, until the egress proxy image is
# published; see below). Idempotent: re-running creates only what is
# missing.
set -euo pipefail

CLUSTER="${1:-cloudbox}"
KIND_PIN=v0.32.0
NODE_IMAGE="kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5"
CALICO_MANIFEST="https://raw.githubusercontent.com/projectcalico/calico/v3.31.0/manifests/calico.yaml"
CANARY_IMAGE="busybox:1.36"  # the enforcement-probe canary (internal/cluster/kube/seal.go)

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/.." && pwd)"

for tool in docker kind kubectl; do
  command -v "$tool" >/dev/null || { echo "$tool is required but not installed" >&2; exit 1; }
done

installed="$(kind version -q 2>/dev/null || kind version | awk '{print $2}')"
if [ "${installed#v}" != "${KIND_PIN#v}" ]; then
  echo "WARNING: kind $installed differs from the tested $KIND_PIN; the node" >&2
  echo "image is digest-pinned, so the cluster is still reproducible." >&2
fi

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  # No --wait: nodes stay NotReady until the CNI below is installed.
  kind create cluster --name "$CLUSTER" --image "$NODE_IMAGE" --config - <<KIND
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true    # Calico below is the enforcing CNI
  podSubnet: 192.168.0.0/16  # calico.yaml's default pool
KIND
fi

if [ -n "${KUBECONFIG_OUT:-}" ]; then
  mkdir -p "$(dirname "$KUBECONFIG_OUT")"
  kind get kubeconfig --name "$CLUSTER" > "$KUBECONFIG_OUT"
  export KUBECONFIG="$KUBECONFIG_OUT"
else
  kind export kubeconfig --name "$CLUSTER" >/dev/null
fi
CONTEXT="kind-$CLUSTER"

# Preload the probe canary so registration's enforcement probe never waits on
# a registry pull. Pulled inside the node with crictl: `kind load` cannot
# import archives from Docker Desktop's containerd image store.
for node in $(kind get nodes --name "$CLUSTER"); do
  docker exec "$node" crictl pull "docker.io/library/$CANARY_IMAGE" >/dev/null
done

# The egress proxy every sealed namespace runs. Until a pinned image is
# published (add-local-agent-mvp task 4.2), it is built from this checkout
# and loaded into the cluster.
"$REPO/hack/conformance/build-proxy-image.sh" "$CLUSTER"

if ! kubectl --context "$CONTEXT" get daemonset calico-node -n kube-system >/dev/null 2>&1; then
  kubectl --context "$CONTEXT" apply -f "$CALICO_MANIFEST" >/dev/null
fi
kubectl --context "$CONTEXT" -n kube-system rollout status daemonset/calico-node --timeout=300s
kubectl --context "$CONTEXT" wait --for=condition=Ready nodes --all --timeout=300s

echo "cluster '$CLUSTER' is ready; kube context: $CONTEXT"
echo "next: register it as a user-controlled sandbox host (see docs/cluster-setup.md)"
