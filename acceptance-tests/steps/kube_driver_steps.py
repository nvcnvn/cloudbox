"""Steps for the kube-driver capability (@conformance: every scenario runs
against a real cluster, booted by run_acceptance.py --conformance)."""

from behave import given, when, then

PRODUCT_CRDS = {
    "applications.cloudbox.dev",
    "sandboxes.cloudbox.dev",
    "bundles.cloudbox.dev",
    "promotionrequests.cloudbox.dev",
    "clusterregistries.cloudbox.dev",
}


# --- Rule: The kube driver satisfies the cluster contract without widening it ---


@given("a reachable Kubernetes cluster with an enforcing CNI")
def step_impl(context):
    assert context.kube.reachable(), (
        "the conformance cluster %r is not reachable" % context.kube.name
    )


@when("cloudboxd starts with the kube driver")
def step_impl(context):
    # The suite booted cloudboxd in before_all; assert it is THIS driver and
    # the process is still alive.
    assert context.driver == "kube", "suite is running driver %r" % context.driver
    assert context.app.poll() is None, "cloudboxd exited"


@then("the control plane reports healthy")
def step_impl(context):
    assert context.platform.healthy(), "GET /healthz did not report healthy"


@then("the cluster contract is served by the kube driver")
def step_impl(context):
    # Registering the real cluster by its kubeconfig context name proves the
    # driver behind the contract resolves real clusters, not simulated ones.
    context.platform.register_cluster(context.kube.name, "sandbox")


@given("cloudboxd running against a real cluster")
def step_impl(context):
    assert context.driver == "kube", "suite is running driver %r" % context.driver
    assert context.platform.healthy(), "control plane is not healthy"


@when("the control plane installs the product custom resource definitions")
def step_impl(context):
    context.platform.run_setup(context.kube.name).raise_for_status()


@then("the definitions are present on the real cluster")
def step_impl(context):
    live = context.kube.crd_names()
    missing = PRODUCT_CRDS - live
    assert not missing, "product CRDs absent from the real cluster: %s" % sorted(missing)


@then("listing them through the driver returns the installed set")
def step_impl(context):
    listed = {crd["name"] for crd in context.platform.installed_crds(context.kube.name)}
    missing = PRODUCT_CRDS - listed
    assert not missing, "driver listing lacks installed CRDs: %s" % sorted(missing)


# --- Rule: The simulation arrangement surface is absent under the kube driver ---


@when("a simulation arrangement route is requested")
def step_impl(context):
    context.sim_arrangement_responses = context.platform.request_sim_arrangements()


@then("the request is not served")
def step_impl(context):
    served = [
        "%s -> %d" % (r.request.url, r.status_code)
        for r in context.sim_arrangement_responses
        if r.status_code != 404
    ]
    assert not served, "simulation arrangement routes served under kube: %s" % served
