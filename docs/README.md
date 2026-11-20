# CloudBox documentation

CloudBox gives a developer or an AI coding agent a sealed, production-shaped
Kubernetes sandbox, and records what happened in it as honest evidence.

## Start here

- [Quickstart](quickstart.md): from no cluster to a sealed sandbox and its
  evidence, using only the command line.
- [Concepts](concepts.md): the terms used everywhere else. Sandbox, seal,
  boundary contract, substrate parity, evidence, and the adoption levels.
- [Limitations](limitations.md): what this release does not do. Read this
  before you rely on it.

## Using it

- [Cluster setup](cluster-setup.md): the cluster CloudBox needs, the
  bootstrap script, registration, and why a cluster is refused.
- [Agent guide](agent-guide.md): the sealed-iteration loop as an agent runs
  it, with every result read as data.
- [CLI reference](cli-reference.md): every command, its flags, the
  machine-readable rendering, and the exit statuses.

## Trust and failure

- [Containment](containment.md): what the seal blocks, what it does not, and
  what it records.
- [Troubleshooting](troubleshooting.md): every refusal and unavailable state,
  what it means, and what to do.
