# Substrate parity changes

The capability's evidence rule is written for the case where production exists:
evidence carries the sandbox's substrate digest and is invalid when it
mismatches production's. It is silent on the case where there is nothing to
compare against — and silence, in an implementation, becomes a default. The
control plane currently reports a substrate match whenever no production
cluster is registered, which is the normal state for every sandbox in a
sandbox-level release. A run that was never compared reports as one that
matched.

That is the quiet-lie failure mode this product exists to eliminate, so parity
gets a third state. The rule below does not weaken the mismatch rule; it adds
the case the mismatch rule never covered. The wording scenario here concerns
the parity fact specifically and does not restate the `evidence` capability's
honest-wording rule, which continues to govern the summary as a whole.

```gherkin
Feature: Substrate parity changes

  # @openspec: ADDED
  Rule: Parity with no registered production is unverified, never matched
    When no production substrate is registered for an application, the control
    plane MUST report that application's substrate parity as unverified. It
    MUST NOT report a match, and no consumer of evidence MAY treat unverified
    parity as a verified one. A comparison that did not happen is not a
    comparison that succeeded.

    Scenario: Evidence records parity as unverified when no production is registered
      Given an application with no registered production substrate
      And a sealed sandbox run for that application
      When the run's evidence is evaluated
      Then the evidence records substrate parity as unverified
      And it does not record a substrate match

    Scenario: The summary states parity was unverified rather than claiming a match
      Given evidence for a run whose application has no registered production substrate
      When the evidence summary is rendered
      Then it states that substrate parity is unverified for want of a registered production substrate
      And it nowhere claims the run was on a substrate matching production

    Scenario: Unverified parity is not a mismatch either
      Given evidence for a run whose application has no registered production substrate
      When the run's evidence is evaluated
      Then the evidence is not marked invalid for substrate mismatch

    Scenario: Registering production resumes comparison
      Given an application whose evidence records substrate parity as unverified
      When a production substrate is registered for it
      Then subsequent evidence records a substrate match or a mismatch rather than unverified
```
