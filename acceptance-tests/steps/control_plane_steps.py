"""Steps for the control-plane capability."""

from behave import given, when, then

EXPECTED_KINDS = {"Application", "Sandbox", "Bundle", "PromotionRequest", "ClusterRegistry"}


@given("a cluster where setup has installed the controller")
def step_impl(context):
    context.cluster = context.platform.ensure_cluster("main")
    context.platform.run_setup(context.cluster).raise_for_status()


@when("the installed CRDs are listed")
def step_impl(context):
    context.crds = context.platform.installed_crds(context.cluster)


@then("exactly Application, Sandbox, Bundle, PromotionRequest, and ClusterRegistry exist under the product's API group")
def step_impl(context):
    kinds = {crd["kind"] for crd in context.crds}
    assert kinds == EXPECTED_KINDS, "expected exactly %s, got %s" % (EXPECTED_KINDS, kinds)
    groups = {crd["group"] for crd in context.crds}
    assert len(context.crds) == 5, "expected 5 CRDs, got %d" % len(context.crds)
    assert len(groups) == 1, "expected one API group, got %s" % groups


@then("user workloads run unwrapped and unmodified beside them")
def step_impl(context):
    context.platform.place_user_workload(context.cluster)
    assert context.platform.user_workload_unmodified(context.cluster), (
        "the user's Deployment was modified or wrapped by the product"
    )


@given("an Application declaring a dependency on an application that does not exist")
def step_impl(context):
    spec = context.applications.draft("web")
    context.app_spec = context.applications.with_dependency(spec, "no-such-app")


@when("the Application is admitted")
def step_impl(context):
    context.applications.admit(context.app_spec)


@then("validation rejects the dangling dependency reference")
def step_impl(context):
    assert context.applications.rejected(), "expected admission to be rejected"
    message = context.applications.rejection_message()
    assert "no-such-app" in message and "dangling" in message.lower(), (
        "rejection should name the dangling reference, got: %s" % message
    )
