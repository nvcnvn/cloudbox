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


# --- CP2: server-side enforcement, thin CLI (task 2.2) ---


@given("a developer running an outdated CLI version")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    # An outdated client performs none of today's client-side niceties and
    # declares an old version; the sim trusts headers the way a real
    # deployment trusts its auth layer.
    context.client_version = "v0.0.1-outdated"


@when("they apply a bundle that current server-side intake rejects")
def step_impl(context):
    context.bundles.last_manifests = context.bundles.cluster_scoped_manifests()
    context.bundles.last_response = context.api.post(
        "/v1/apply",
        json={
            "app": context.app_name,
            "sandbox": context.sandbox_name,
            "manifests": context.bundles.last_manifests,
        },
        headers={
            "X-Cloudbox-User": "dev@example.com",
            "X-Cloudbox-Client-Version": context.client_version,
        },
    )


@then("the apply is rejected by the controller regardless of the client version")
def step_impl(context):
    assert context.bundles.rejected(), (
        "server-side intake must reject no matter the client, got %s"
        % context.bundles.last_response.status_code
    )
    findings = context.bundles.findings()
    assert any(f["code"] == "cluster-scoped-resource" for f in findings), findings


@given("a manifest directory that passes an outdated offline check")
def step_impl(context):
    # The offline check has no boundary contract, so a contract-dependent
    # violation passes it — the structural way any offline/outdated copy of
    # intake analysis under-reports.
    context.manifests = context.bundles.secret_mounting_manifests("payment-api-key")
    context.check_dir = context.check.write_directory({"app.yaml": context.manifests})
    context.check.run(context.check_dir)
    assert context.check.exit_code() == 0, (
        "fixture must pass the offline check, got:\n%s" % context.check.result.output
    )


@when("the bundle is applied to a sandbox")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)


@then("server-side intake re-runs the analysis and its verdict is authoritative")
def step_impl(context):
    assert context.bundles.rejected(), (
        "server-side intake must reject despite the passing offline check"
    )
    findings = context.bundles.findings()
    assert any(f["code"] == "undeclared-secret" for f in findings), findings
