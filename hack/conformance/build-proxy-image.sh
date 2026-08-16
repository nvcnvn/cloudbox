#!/usr/bin/env bash
# Build the product egress proxy image and load it into a Kind cluster.
# Usage: build-proxy-image.sh <kind-cluster-name>
set -euo pipefail

CLUSTER="${1:?kind cluster name required}"
IMAGE="cloudbox-egress-proxy:dev"

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
BUILD_DIR="$REPO/bin/proxy-image"

# The node's architecture decides the binary's; ask the node itself.
node="$(kind get nodes --name "$CLUSTER" | head -1)"
arch="$(docker exec "$node" uname -m)"
case "$arch" in
  aarch64) goarch=arm64 ;;
  x86_64)  goarch=amd64 ;;
  *) echo "unsupported node architecture: $arch" >&2; exit 1 ;;
esac

mkdir -p "$BUILD_DIR"
(cd "$REPO" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" \
  go build -o "$BUILD_DIR/cloudbox-proxy" ./cmd/cloudbox-proxy)
cp "$HERE/proxy.Dockerfile" "$BUILD_DIR/Dockerfile"
docker build -q -t "$IMAGE" "$BUILD_DIR" >/dev/null

# kind load chokes on Docker Desktop's containerd image store; fall back to a
# manual ctr import without digest verification.
if ! kind load docker-image "$IMAGE" --name "$CLUSTER" 2>/dev/null; then
  echo "kind load failed; importing via ctr" >&2
  docker save "$IMAGE" | docker exec -i "$node" \
    ctr --namespace=k8s.io images import --all-platforms -
fi
echo "egress proxy image $IMAGE loaded into $CLUSTER"
