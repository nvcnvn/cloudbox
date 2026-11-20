# Control plane changes

The capability already fixes where authority lives: all validation, bundling,
evidence gathering, signing, and enforcement run server-side, and the CLI is
thin so there is no client-version drift in enforcement. Nothing about that
changes here. What changes is who is allowed to start the server.

An agent iterating locally should run one binary, not two. Letting the command
line start a control plane and reuse it across commands removes a process the
agent would otherwise have to supervise, without moving a single decision out
of the control plane: the started process is the server, and it enforces
exactly what a separately operated one enforces. The rule is stated explicitly
because the alternative — linking the control plane into the CLI for local use
— would create a second execution model and put enforcement back on the client,
which ADR 0004 forbids.

```gherkin
Feature: Control plane changes

  # @openspec: ADDED
  Rule: The command line may start and reuse a local control plane
    The CLI MUST be able to start a control plane on the developer's machine
    when none is addressed, and MUST reuse it across subsequent commands
    rather than starting another. Enforcement MUST remain server-side in the
    started control plane; the CLI MUST NOT acquire validation, bundling, or
    evidence authority by virtue of having started it.

    Scenario: The first command starts a control plane and later commands reuse it
      Given a developer machine with no control plane running
      When two commands requiring the control plane are run in turn
      Then the first starts a control plane
      And the second uses the already-running one rather than starting another

    Scenario: A CLI-started control plane enforces exactly as an operated one
      Given a bundle that server-side intake rejects
      When it is applied through a CLI-started control plane
      Then it is rejected on the same grounds as when applied through a separately operated control plane

    Scenario: An addressed control plane is used instead of starting one
      Given a developer who addresses a running control plane explicitly
      When they run a command requiring the control plane
      Then the addressed control plane serves the request
      And no local control plane is started
```
