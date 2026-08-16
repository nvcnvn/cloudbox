"""Steps for the conformance-ci capability's untagged (sim-run) scenarios."""

from behave import given, when, then

from pages.gate import check_enforcement


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
