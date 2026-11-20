#!/usr/bin/env bash
# Provision the enforcing conformance cluster (ADR 0008) with the user-facing
# bootstrap, so the conformance target and the cluster a user is told to make
# are the same cluster by construction.
#
# Calico is installed explicitly as the enforcing CNI. kind v0.32.0's default
# CNI was VERIFIED to enforce NetworkPolicy (see README.md), but the
# conformance claim never rides on a default that the next version bump could
# change.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"

KUBECONFIG_OUT="$REPO/acceptance-tests/.kube/conformance.kubeconfig" \
  "$REPO/scripts/kind-cluster.sh" cloudbox-conformance

echo "enforcing conformance cluster 'cloudbox-conformance' ready"
echo "kubeconfig: $REPO/acceptance-tests/.kube/conformance.kubeconfig"
