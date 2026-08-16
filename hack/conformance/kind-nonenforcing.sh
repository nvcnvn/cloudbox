#!/usr/bin/env bash
# Provision the DELIBERATELY NON-ENFORCING cluster (ADR 0008, design Open
# Question 3): Kind with flannel, a CNI that accepts NetworkPolicy objects
# without enforcing them. This cluster exists to prove the product refuses to
# lie — the probe-failure and enforcement-gate scenarios assert against it.
#
# Stock Kind cannot serve this role: kind v0.32.0's default kindnetd was
# VERIFIED to enforce NetworkPolicy (see README.md), so non-enforcement is
# arranged explicitly with a CNI that never implemented it.
#
# Idempotent, like kind-enforcing.sh.
set -euo pipefail

CLUSTER=cloudbox-nonenforcing
NODE_IMAGE="kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5"
FLANNEL_MANIFEST="https://raw.githubusercontent.com/flannel-io/flannel/v0.27.4/Documentation/kube-flannel.yml"
CANARY_IMAGE="busybox:1.36"

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
KUBECONFIG_OUT="$REPO/acceptance-tests/.kube/nonenforcing.kubeconfig"

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  kind create cluster --name "$CLUSTER" --image "$NODE_IMAGE" --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true   # flannel below — kindnetd would enforce
  podSubnet: 10.244.0.0/16  # flannel's default pool
EOF
fi

mkdir -p "$(dirname "$KUBECONFIG_OUT")"
kind get kubeconfig --name "$CLUSTER" > "$KUBECONFIG_OUT"
export KUBECONFIG="$KUBECONFIG_OUT"

# kindest/node ships without the reference CNI plugins flannel delegates to
# ("bridge" and friends); install them on each node.
CNI_PLUGINS_VERSION=v1.6.2
for node in $(kind get nodes --name "$CLUSTER"); do
  if docker exec "$node" test -x /opt/cni/bin/bridge; then
    continue
  fi
  arch="$(docker exec "$node" uname -m)"
  case "$arch" in
    aarch64) goarch=arm64 ;;
    x86_64)  goarch=amd64 ;;
    *) echo "unsupported node architecture: $arch" >&2; exit 1 ;;
  esac
  tarball="$(mktemp)"
  curl -sL -o "$tarball" \
    "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${goarch}-${CNI_PLUGINS_VERSION}.tgz"
  # /tmp inside the node is a tmpfs that shadows docker-cp writes; use /root.
  docker cp "$tarball" "$node":/root/cni-plugins.tgz
  docker exec "$node" tar -C /opt/cni/bin -xzf /root/cni-plugins.tgz
  docker exec "$node" rm -f /root/cni-plugins.tgz
  rm -f "$tarball"
done

if ! kubectl get daemonset kube-flannel-ds -n kube-flannel >/dev/null 2>&1; then
  kubectl apply -f "$FLANNEL_MANIFEST"
fi
kubectl -n kube-flannel rollout status daemonset/kube-flannel-ds --timeout=300s
kubectl wait --for=condition=Ready nodes --all --timeout=300s

for node in $(kind get nodes --name "$CLUSTER"); do
  docker exec "$node" crictl pull "docker.io/library/$CANARY_IMAGE"
done
"$HERE/build-proxy-image.sh" "$CLUSTER"

echo "non-enforcing cluster '$CLUSTER' ready (flannel: NetworkPolicy accepted, not enforced)"
echo "kubeconfig: $KUBECONFIG_OUT"
