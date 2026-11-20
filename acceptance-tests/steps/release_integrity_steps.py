"""Steps for the release-integrity capability (add-local-agent-mvp, ADR 0009)."""

from behave import given, when, then

from pages.release import GITOPS, PRODUCTION_INGEST, SOURCE_CONTROL

DEV = "dev@example.com"
PR = "42"


def _valid_run(context):
    """A run every other gate would pass, so a refusal can only come from the
    missing integration."""
    context.app_name = context.applications.create("web", testSuite={"name": "smoke", "tests": 3})["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()


# --- An operation with no implemented integration refuses (task 1.4) ---


@given("a released control plane with no source-control or GitOps integration")
def step_impl(context):
    _valid_run(context)
    context.release.withdraw_integrations(SOURCE_CONTROL, GITOPS)
    context.missing_integration = GITOPS


@given("a released control plane with no source-control integration")
def step_impl(context):
    _valid_run(context)
    context.release.withdraw_integrations(SOURCE_CONTROL)
    context.missing_integration = SOURCE_CONTROL


@given("a released control plane with no production ingest")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.release.withdraw_integrations(PRODUCTION_INGEST)
    context.missing_integration = PRODUCTION_INGEST


@when("a promotion is opened")
def step_impl(context):
    context.release.open_promotion(context.sandbox_name)


@when("an evidence check is posted for a pull request")
def step_impl(context):
    context.release.post_evidence_check(context.sandbox_name, PR)


@when("production state is set for an application")
def step_impl(context):
    context.release.set_production_state(context.app_name, context.bundles.plain_mixed_manifests())


@then("the request is refused naming the missing integration")
def step_impl(context):
    status, message = context.release.refusal()
    assert 400 <= status < 600, (status, message)
    assert context.missing_integration.lower() in message.lower(), (
        "refusal does not name %r: %s" % (context.missing_integration, message)
    )
    assert "integration" in message.lower(), message


@then("no promotion record is created")
def step_impl(context):
    assert context.release.promotions(context.app_name) == []


@then("no status check is recorded")
def step_impl(context):
    assert context.release.checks(context.app_name, PR) == []


@then("no production state is recorded for that application")
def step_impl(context):
    assert "liveDigest" not in context.release.production_status(context.app_name)
