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


# --- The released binaries identify the build they came from (task 4.4) ---

from pathlib import Path

from pages.release import (
    TEST_PROXY_IMAGE,
    TEST_RELEASE_VERSION,
    ReleasedControlPlane,
    release_build,
    version_report,
)


@given("a binary produced by the project's release process")
def step_impl(context):
    out = release_build()
    context.version_binaries = [out / "cloudbox", out / "cloudboxd"]


@given("a binary built locally rather than by the release process")
def step_impl(context):
    local = Path(context.cli_path).parent
    context.version_binaries = [local / "cloudbox", local / "cloudboxd"]


@when("its version is requested")
def step_impl(context):
    context.version_reports = [version_report(b) for b in context.version_binaries]


@then("it reports the version identifying that build")
def step_impl(context):
    for report in context.version_reports:
        assert report["version"] == TEST_RELEASE_VERSION and report["released"], report


@then("the report identifies it as an unreleased build")
def step_impl(context):
    for report in context.version_reports:
        assert "unreleased" in report["version"], report


@then("it does not present itself as a released version")
def step_impl(context):
    for report in context.version_reports:
        assert report["released"] is False, report


# --- The egress proxy is deployed from a pinned, resolvable reference (task 4.2) ---


@given("a registered cluster with no product images preloaded")
def step_impl(context):
    cp = ReleasedControlPlane()
    context.add_cleanup(cp.stop)
    cp.post("/simctl/clusters", {"name": "fresh", "enforcing": True})
    cp.post("/v1/setup", {"cluster": "fresh"})
    cp.post("/v1/clusters/register", {"cluster": "fresh", "role": "sandbox", "userControlled": True})
    context.released_cp = cp


@when("a sandbox is sealed")
def step_impl(context):
    cp = context.released_cp
    cp.post("/v1/applications", {"name": "web", "owners": [DEV]})
    sandbox = cp.post("/v1/sandboxes", {"app": "web"}, headers={"X-Cloudbox-User": DEV})
    context.released_sandbox = cp.get("/v1/sandboxes/%s" % sandbox["name"])


@then("the egress proxy is deployed from the release's pinned image reference")
def step_impl(context):
    proxy = context.released_sandbox["egressProxy"]
    assert proxy["image"] == TEST_PROXY_IMAGE, proxy
    assert "@sha256:" in proxy["image"], proxy


# --- A sandbox whose proxy cannot be provisioned is never allowlist-enforcing (task 4.3) ---

DECLARED_ENDPOINT = "api.stripe.com"


def _sandbox_without_registry(context):
    cluster = context.platform.ensure_cluster("no-registry", enforcing=True)
    context.platform.run_setup(cluster).raise_for_status()
    context.platform.make_registry_unreachable(cluster)
    context.platform.register_user_controlled_host(cluster)
    context.app_name = context.applications.create(
        "web", sandboxCluster=cluster,
        contract={"secretNames": [], "ingressHostnames": [], "egressAllowlist": [DECLARED_ENDPOINT], "dependencies": []},
    )["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, arrange=False)
    context.sandbox_record = context.sandboxes.record(context.sandbox_name).json()


@given("a cluster that cannot resolve the release's egress proxy image")
def step_impl(context):
    context.unresolvable = True


@when("a sandbox is created on it")
def step_impl(context):
    _sandbox_without_registry(context)


@then("the sandbox reports the proxy unavailable naming the unresolvable reference")
def step_impl(context):
    proxy = context.sandbox_record["egressProxy"]
    assert proxy["state"] == "unavailable", proxy
    assert proxy["image"] and proxy["image"] in proxy["reason"], proxy


@then("the sandbox is not reported as enforcing its allowlist")
def step_impl(context):
    assert context.sandbox_record["allowlistEnforced"] is False, context.sandbox_record


@given("a sandbox whose egress proxy could not be provisioned")
def step_impl(context):
    _sandbox_without_registry(context)


@given("an external endpoint on the application's declared allowlist")
def step_impl(context):
    contract = context.applications.get(context.app_name)["contract"]
    assert DECLARED_ENDPOINT in contract["egressAllowlist"], contract


@when("a workload attempts that declared endpoint")
def step_impl(context):
    resp = context.api.post(
        "/simctl/sandboxes/%s/egress-attempts" % context.sandbox_name,
        json={"workload": "web", "destination": DECLARED_ENDPOINT},
    )
    resp.raise_for_status()
    context.attempt = resp.json()


@then("the attempt is denied rather than allowed unmediated")
def step_impl(context):
    assert context.attempt["allowed"] is False, context.attempt
    assert context.attempt.get("via") != "unfiltered", context.attempt
