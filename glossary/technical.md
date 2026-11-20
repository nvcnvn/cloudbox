# Technical glossary

Architecture, implementation, platform, and protocol terms. Not rendered for
users.

| Term | Definition | Use When | Avoid |
| --- | --- | --- | --- |
| Control plane | The `cloudboxd` server that holds all enforcement and mints all evidence (ADR 0004). | Naming the authority the CLI talks to. | "backend", "daemon" when authority is meant |
| Enforcing CNI | A cluster network plugin that enforces Kubernetes NetworkPolicy, such as Calico. | Stating the cluster requirement. | "CNI" alone, which does not imply enforcement |
| Egress proxy | The product-deployed proxy that admits allowlisted HTTP egress and records attempts by FQDN. | Describing how allowlisted egress is permitted and recorded. | "sidecar" |
| Machine-readable rendering | A command's structured output, carrying the same facts as its human rendering from the same server response. | Describing output an agent consumes. | "JSON API mode" (implies a second surface) |
| Exit status | The process status that distinguishes success, a refusal, and an unreachable control plane. | Describing outcomes a program branches on. | "return code" |
| Effective spec | The source-of-truth specs plus active change deltas; the default acceptance run. | Naming what `make acceptance` runs. | "the specs" when deltas are included |
| Conformance subset | The `@conformance`-tagged scenarios run against real Kind clusters. | Naming the real-cluster test run. | "integration tests" |
