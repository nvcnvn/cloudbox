"""Steps for the conformance-ci capability: the untagged (sim-run) scenarios
plus the @conformance real-clock lifecycle scenarios."""

import time

from behave import given, when, then

from pages.bundles import SHORTNAME_NO_SERVICE_YAML, SHORTNAME_WITH_SERVICE_YAML
from pages.gate import check_enforcement
from kube_driver_steps import create_sealed_sandbox


# --- Rule: The default acceptance run excludes conformance-tagged scenarios ---


@given("a working copy with no Kubernetes cluster available")
def step_impl(context):
    # Nothing to arrange: the runner inspection below never contacts a
    # cluster, which is exactly the property under test.
    pass


@when("the default acceptance run is invoked")
def step_impl(context):
    context.run_selection = context.runner.dry_run_default()


@then("the run completes without attempting any conformance-tagged scenario")
def step_impl(context):
    tagged = context.runner.conformance_tagged_scenarios()
    assert tagged, "no conformance-tagged scenarios exist in the extracted tree"
    attempted = context.run_selection.attempted() & tagged
    assert not attempted, (
        "the default run attempted conformance-tagged scenarios: %s"
        % sorted(attempted)
    )


@then("no conformance-tagged scenario is reported as a failure")
def step_impl(context):
    tagged = context.runner.conformance_tagged_scenarios()
    failed = context.run_selection.failures() & tagged
    assert not failed, (
        "conformance-tagged scenarios reported as failures: %s" % sorted(failed)
    )


@given("the acceptance suite with its conformance-tagged scenarios")
def step_impl(context):
    assert context.runner.conformance_tagged_scenarios(), (
        "no conformance-tagged scenarios exist in the extracted tree"
    )


@when("the conformance run is invoked")
def step_impl(context):
    context.run_selection = context.runner.dry_run_conformance()


@then("only conformance-tagged scenarios are selected")
def step_impl(context):
    attempted = context.run_selection.attempted()
    assert attempted, "the conformance run selected nothing"
    untagged = attempted - context.runner.conformance_tagged_scenarios()
    assert not untagged, (
        "the conformance run selected untagged scenarios: %s" % sorted(untagged)
    )


# --- Rule: A conformance run gates on proven NetworkPolicy enforcement ---
#
# The gate's logic (pages/gate.py) is exercised here in both directions; a
# real conformance run executes the same gate in before_all against the real
# target cluster, so these scenarios and the live gate cannot drift apart.


@given("a real cluster whose CNI enforces NetworkPolicy")
def step_impl(context):
    if context.driver == "kube":
        context.gate_target = context.kube.name
        return
    context.gate_target = context.platform.ensure_cluster("enforcing-target", enforcing=True)


@given("a real cluster whose CNI does not enforce NetworkPolicy")
def step_impl(context):
    if context.driver == "kube":
        context.gate_target = context.kube_nonenforcing.name
        return
    context.gate_target = context.platform.ensure_cluster(
        "nonenforcing-target", enforcing=False
    )


@when("the conformance run checks its enforcement precondition")
def step_impl(context):
    context.gate_passed, context.gate_message = check_enforcement(
        context.api, context.gate_target
    )


@then("the precondition passes and the run proceeds")
def step_impl(context):
    assert context.gate_passed, (
        "the enforcement precondition failed: %s" % context.gate_message
    )


@then("the run fails naming unproven network policy enforcement")
def step_impl(context):
    assert not context.gate_passed, "the gate passed on a non-enforcing cluster"
    assert "unproven network policy enforcement" in context.gate_message, (
        context.gate_message
    )


@then("the run reports no conformance pass")
def step_impl(context):
    assert "no conformance pass" in context.gate_message, context.gate_message


# --- Rule: Conformance lifecycle expiry uses the real clock ---
#
# No simulated clock exists on this path: /simctl/advance-time is never
# served under the kube driver (proven by its own scenario), so the only way
# these pass is the wall clock actually elapsing.


def wait_until_destroyed(context, timeout=120):
    deadline = time.time() + timeout
    record = None
    while time.time() < deadline:
        record = context.sandboxes.record(context.sandbox_name).json()
        if record["state"] == "destroyed":
            return record
        time.sleep(2)
    raise AssertionError(
        "sandbox %r was not destroyed in real time; last state %r"
        % (context.sandbox_name, record and record["state"])
    )


@given("a sandbox on a real cluster with a time-to-live of a few seconds")
def step_impl(context):
    create_sealed_sandbox(context, ttl_seconds=20)


@when("that time-to-live elapses in real time")
def step_impl(context):
    # The shared "the sandbox is destroyed" Then re-reads the record; this
    # wait is what makes it real time rather than a simulated advance.
    context.final_record = wait_until_destroyed(context)


@then("its namespace is removed from the cluster")
def step_impl(context):
    deadline = time.time() + 120
    while time.time() < deadline:
        if not context.kube.namespace_exists(context.sandbox_name):
            return
        time.sleep(2)
    raise AssertionError(
        "namespace %r still exists on the cluster" % context.sandbox_name
    )


@given("a sandbox on a real cluster with an idle-expiry window of a few seconds")
def step_impl(context):
    create_sealed_sandbox(context, app_fields={"policies": {"idleExpirySeconds": 10}})


@when("the sandbox sees no activity for longer than that window")
def step_impl(context):
    context.final_record = wait_until_destroyed(context)


# --- Rule: Scenarios the real run cannot honestly prove are excluded and recorded ---


@given("the defined conformance subset")
def step_impl(context):
    assert context.conformance_subset.subset(), "the conformance subset is empty"


@when("its scenarios are listed")
def step_impl(context):
    context.listed_subset = context.conformance_subset.subset()


@then("no soak-window scenario is present")
def step_impl(context):
    soaked = context.conformance_subset.soak_scenarios_in_subset()
    assert not soaked, "soak-window scenarios in the subset: %s" % sorted(soaked)


@then("the exclusion reason is recorded with the subset definition")
def step_impl(context):
    records = context.conformance_subset.recorded_exclusions()
    soak_records = [
        r for r in records
        if "soak" in r["excluded"].lower() and r["reason"].strip()
    ]
    assert soak_records, (
        "the subset definition records no soak exclusion reason (records: %s)"
        % [r["excluded"] for r in records]
    )


@then("no scenario arranged only through a simulated external system is present")
def step_impl(context):
    flagged = context.conformance_subset.externally_arranged_in_subset()
    assert not flagged, (
        "subset scenarios arranged through simulated external systems: %s"
        % sorted(flagged)
    )


# --- Rule: Continuous integration runs the full effective suite on every change ---


@given("the continuous-integration configuration")
def step_impl(context):
    assert context.ci_config.exists(), "no continuous-integration workflow exists"


@when("its required stages are read")
def step_impl(context):
    context.missing_stages = context.ci_config.missing_stages()


@then("it builds both binaries, vets the sources, lints the extracted specifications, runs the simulation suite, and runs the conformance subset")
def step_impl(context):
    assert not context.missing_stages, (
        "continuous integration is missing required stages: %s"
        % context.missing_stages
    )


@given("a continuous-integration run whose conformance subset fails")
def step_impl(context):
    context.failing_run = context.ci_config.run_conformance_against_broken_cluster()


@when("the check result is reported")
def step_impl(context):
    # A CI step's verdict is its exit code, and the workflow must not swallow
    # one (no continue-on-error, no shell-level masking).
    context.check_fails = (
        context.failing_run.returncode != 0
        and context.ci_config.propagates_failures()
    )


@then("the check fails")
def step_impl(context):
    assert context.check_fails, (
        "a failing conformance subset did not fail the check (exit=%d):\n%s"
        % (context.failing_run.returncode, context.failing_run.stderr[-2000:])
    )


# --- Rule: A divergence between the simulation and the real driver is recorded and reconciled ---


@given("a behaviour where the real driver contradicts the simulation")
def step_impl(context):
    # The flagship finding: the sim resolved in-sandbox short names against
    # workload names; real cluster DNS resolves Service names (task 3.7).
    records = context.sim_divergences.records()
    context.divergence = next(
        (r for r in records if "Service" in r["behaviour"]), None
    )
    assert context.divergence, (
        "no recorded divergence covers short-name/Service resolution: %s"
        % [r["id"] for r in records]
    )


@when("that divergence is reconciled")
def step_impl(context):
    # Exercise the corrected model in both directions on the sim.
    context.app_name = context.applications.create("recon")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.bundles.apply(context.app_name, context.sandbox_name, SHORTNAME_NO_SERVICE_YAML)
    context.bundles.last_response.raise_for_status()
    context.without_service = context.sandboxes.attempt_egress(
        context.sandbox_name, "auth-api"
    )
    context.bundles.apply(context.app_name, context.sandbox_name, SHORTNAME_WITH_SERVICE_YAML)
    context.bundles.last_response.raise_for_status()
    context.with_service = context.sandboxes.attempt_egress(
        context.sandbox_name, "auth-api"
    )


@then("the simulation is corrected to match the real driver")
def step_impl(context):
    # Real behaviour (task 3.7): no Service, no resolution; with a Service,
    # the in-sandbox connection succeeds under the seal.
    assert not context.without_service["allowed"], (
        "the sim still resolves a bare workload name: %s" % context.without_service
    )
    assert context.with_service["allowed"] and context.with_service["via"] == "in-sandbox", (
        context.with_service
    )


@then("the divergence is recorded with the behaviour that differed")
def step_impl(context):
    behaviour = context.divergence["behaviour"]
    correction = context.divergence["correction"]
    assert "sim" in behaviour and "real" in behaviour, (
        "the record does not describe both sides of the behaviour: %r" % behaviour
    )
    assert correction.strip(), "the record carries no correction"
