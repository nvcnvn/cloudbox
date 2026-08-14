# 0001 — Standard NetworkPolicy floor with a product egress proxy

- Status: accepted
- Date: 2026-08-14
- Supersedes: —

## Context

The seal (default-deny ingress/egress with an FQDN allowlist) needs an
enforcement mechanism. Candidates: vendor policy CRDs (Cilium/Calico), a
service mesh, eBPF-only enforcement, or standard Kubernetes NetworkPolicy v1.
FQDN allowlisting is not expressible in NetworkPolicy v1 alone, and a CNI that
does not enforce NetworkPolicy fails silently (policies accepted, ignored).

## Decision

The seal's enforcement floor is standard NetworkPolicy v1 — no vendor policy
CRDs, no specific CNI, no mesh. Default-deny admits only cluster DNS and the
product-managed egress proxy, which enforces the FQDN allowlist; transparent
pod-level redirection (iptables/DNS-interception helper) seals non-HTTP and
proxy-unaware workloads without workload modification. Enforcement is
probe-verified at setup and sandbox creation: an unverified seal never reports
sealed and never produces evidence. Where a DNS-aware CNI is detected, the
allowlist MAY compile to native policy — same contract, better mechanism.

## Consequences

- The product runs on any conformant Kubernetes ≥ 1.29 whose CNI enforces
  NetworkPolicy v1 (major managed offerings, default k3s, Kind).
- The egress proxy is a mandatory, product-managed component; there is no
  route around it by construction.
- Containment claims are bounded: residual channels (DNS tunneling,
  exfiltration via allowlisted endpoints) exist in v1 and must be published,
  never papered over ("unbypassable" is banned vocabulary).
- The redirection helper needs a conformance matrix across CNIs/runtimes.
