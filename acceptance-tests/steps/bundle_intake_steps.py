"""Steps for the bundle-intake capability."""

from behave import given, when, then


def _fresh_app_and_sandbox(context, app_name="web"):
    context.app_name = context.applications.create(app_name)["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)


@given("a manifest directory containing a Deployment, a Service, and a custom resource")
def step_impl(context):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.plain_mixed_manifests()


@given("none of the manifests carry any CloudBox-specific field or annotation")
def step_impl(context):
    assert "cloudbox" not in context.manifests.lower()


@when("the developer applies the directory to their sandbox")
def step_impl(context):
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)


@then("the apply is accepted")
def step_impl(context):
    assert context.bundles.accepted(), (
        "expected the apply to be accepted, got %s: %s"
        % (context.bundles.last_response.status_code, context.bundles.last_response.text)
    )
