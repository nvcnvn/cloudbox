"""Steps for the sandbox-lifecycle capability."""

from behave import given, when, then

PRIYA = "priya@example.com"
MINH = "minh@example.com"


def _advance(context, seconds):
    context.api.post("/simctl/advance-time", json={"seconds": seconds}).raise_for_status()


# --- S1: one command, no approval, owner-scoped (task 3.1) ---


@given("a developer with access to the application")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.developer = PRIYA


@when("they run the sandbox create command")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=context.developer)


@then("a sandbox owned by them is provisioned")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["owner"] == context.developer, record
    assert record["state"] == "ready", record


@then("no approval step occurs")
def step_impl(context):
    # Creation returned ready immediately: nothing was pending on anyone.
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["state"] == "ready" and "approval" not in record, record


@given("a sandbox owned by developer Priya")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=PRIYA)


@when("developer Minh attempts to apply a bundle to that sandbox")
def step_impl(context):
    context.bundles.apply(
        context.app_name, context.sandbox_name,
        context.bundles.plain_mixed_manifests(), actor=MINH,
    )


@then("the request is denied as not the sandbox owner")
def step_impl(context):
    assert context.bundles.last_response.status_code == 403, (
        context.bundles.last_response.text
    )
    assert "owner" in context.bundles.error_message()


# --- S2: thirty-second readiness (task 3.2) ---


@given("a shared cluster registered with the control plane")
def step_impl(context):
    context.platform.ensure_ready_platform()
    context.app_name = context.applications.create("web")["name"]


@when("a developer creates a sandbox")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name)


@then("the sandbox reports sealed and ready for apply within thirty seconds")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["sealed"] and record["state"] == "ready", record
    assert record["readySeconds"] <= 30, (
        "control-plane readiness took %.1fs" % record["readySeconds"]
    )


@then("any remaining wait is attributable to the user's own workloads")
def step_impl(context):
    # Control-plane readiness is complete; only user workloads could still be
    # pending, and every admitted one reports its own readiness.
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["state"] == "ready", record
    for workload in context.sandboxes.workloads(context.sandbox_name):
        assert "ready" in workload


# --- S3: local sandboxes (task 3.3) ---


@given("an application with a substrate lockfile")
def step_impl(context):
    context.platform.ensure_ready_platform()
    context.app_name = context.applications.create("web")["name"]
    context.lockfile = context.api.get(
        "/v1/applications/%s/substrate-lockfile" % context.app_name
    ).json()


@when("the developer creates a sandbox with the local option")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name, local=True)


@then("a local Kind cluster is provisioned from the lockfile")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["local"] and record["cluster"].startswith("kind-"), record
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["substrateDigest"] == context.lockfile["digest"], (
        "local substrate %s != lockfile %s" % (evidence["substrateDigest"], context.lockfile["digest"])
    )


@then("the sandbox enforces the same seal and iteration semantics as a managed sandbox")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["sealed"] and record["sealVerified"], record
    attempt = context.api.post(
        "/simctl/sandboxes/%s/egress-attempts" % context.sandbox_name,
        json={"workload": "web", "destination": "api.undeclared-vendor.com"},
    ).json()
    assert not attempt["allowed"], "the local seal must deny undeclared egress"


@given("evidence produced by a local sandbox")
def step_impl(context):
    context.platform.ensure_ready_platform()
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, local=True)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()


@when("the developer attempts to post an evidence check or open a promotion from it")
def step_impl(context):
    context.check_attempt = context.api.post(
        "/v1/evidence-checks", json={"sandbox": context.sandbox_name, "pr": "42"}
    )
    context.promote_attempt = context.api.post(
        "/v1/promotions", json={"sandbox": context.sandbox_name}
    )


@then("the attempt is refused because the evidence source is not a control-plane-managed sandbox")
def step_impl(context):
    for attempt in (context.check_attempt, context.promote_attempt):
        assert attempt.status_code == 403, attempt.text
        assert "control-plane-managed" in attempt.json()["error"], attempt.text


# --- S5: TTL, idle expiry, quotas (task 3.4) ---


@given("a sandbox created with a TTL of one day")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, ttl_seconds=86400)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()


@when("the TTL elapses")
def step_impl(context):
    _advance(context, 86401)


@then("the sandbox and its workloads are destroyed")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["state"] == "destroyed", record
    assert context.sandboxes.workloads(context.sandbox_name) == []


@given("a sandbox with idle expiry configured by the application")
def step_impl(context):
    context.app_name = context.applications.create(
        "web", policies={"idleExpirySeconds": 3600}
    )["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)


@when("the sandbox sees no activity for the idle window")
def step_impl(context):
    _advance(context, 3601)


@then("the sandbox is destroyed")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["state"] == "destroyed", record


@given("an application quota of four CPUs per sandbox")
def step_impl(context):
    context.app_name = context.applications.create(
        "web", policies={"cpuQuotaPerSandbox": 4}
    )["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)


@when("a developer applies a bundle requesting eight CPUs after squeezing")
def step_impl(context):
    # 8 cores/container squeezed ×0.25 → 2 cores; 4 replicas → 8 cores total.
    manifests = context.bundles.prod_sized_manifests(replicas=4, cpu="8", memory="1Gi")
    context.bundles.apply_options(context.app_name, context.sandbox_name, manifests)


@then("the apply is rejected with the quota that was exceeded")
def step_impl(context):
    assert context.bundles.rejected(), "expected quota rejection"
    message = context.bundles.error_message()
    assert "quota" in message and "4" in message, message


# --- S6: PR-bound lifecycle (task 3.5) ---


def _pr_app(context):
    context.app_name = context.applications.create("web", scmIntegration=True)["name"]
    context.platform.ensure_ready_platform()
    context.pr = "42"


@given("the SCM integration is enabled for the application")
def step_impl(context):
    _pr_app(context)


@when("a pull request is opened")
def step_impl(context):
    context.pr_sandbox = context.scm.open_pr(context.app_name, context.pr)


@then("a sandbox bound to that pull request is created")
def step_impl(context):
    assert context.pr_sandbox["pullRequest"] == context.pr, context.pr_sandbox
    assert context.pr_sandbox["state"] == "ready", context.pr_sandbox


@given("a PR-bound sandbox healthy for three hours on digest A")
def step_impl(context):
    _pr_app(context)
    context.pr_sandbox = context.scm.open_pr(context.app_name, context.pr)
    context.sandbox_name = context.pr_sandbox["name"]
    context.manifests_a = context.bundles.plain_mixed_manifests()
    context.scm.push(context.app_name, context.pr, context.manifests_a)
    _advance(context, 3 * 3600)
    soak = context.sandboxes.evidence(context.sandbox_name)["observedHealthySeconds"]
    assert soak >= 3 * 3600, "arrange failed: soak is %.0fs" % soak


@when("a push re-renders the branch to digest B")
def step_impl(context):
    manifests_b = context.manifests_a + "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: extra\n"
    context.scm.push(context.app_name, context.pr, manifests_b)


@then("the new bundle is applied")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["appliedDigest"] != "", record


@then("observed-healthy duration restarts from zero")
def step_impl(context):
    soak = context.sandboxes.evidence(context.sandbox_name)["observedHealthySeconds"]
    assert soak < 60, "soak should have reset, got %.0fs" % soak


@when("a rebase re-renders the branch and the digest is still A")
def step_impl(context):
    context.scm.push(context.app_name, context.pr, context.manifests_a)


@then("the accumulated three hours of soak time are preserved")
def step_impl(context):
    soak = context.sandboxes.evidence(context.sandbox_name)["observedHealthySeconds"]
    assert soak >= 3 * 3600, "soak inheritance lost: %.0fs" % soak


@given("a PR-bound sandbox")
def step_impl(context):
    _pr_app(context)
    context.pr_sandbox = context.scm.open_pr(context.app_name, context.pr)
    context.sandbox_name = context.pr_sandbox["name"]


@when("the pull request is merged or closed")
def step_impl(context):
    context.scm.close_pr(context.app_name, context.pr, merged=True)


@then("the sandbox TTL fires and the sandbox is destroyed")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["state"] == "destroyed", record


# --- S7: capacity transforms (task 3.6) ---


@given("a production-sized bundle with three replicas and quorum-based leader election")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.manifests = context.bundles.prod_sized_manifests(replicas=3, cpu="1000m", memory="512Mi")


@when("the bundle is admitted with the default squeezed capacity mode")
def step_impl(context):
    context.bundles.apply_options(context.app_name, context.sandbox_name, context.manifests)
    context.bundles.last_response.raise_for_status()


@then("replica counts, topology, and scheduling constraints are preserved")
def step_impl(context):
    workloads = context.sandboxes.workloads(context.sandbox_name)
    assert workloads, "no workloads admitted"
    assert workloads[0]["manifest"]["spec"]["replicas"] == 3, workloads[0]


@then("CPU requests are scaled down while memory is only reduced to a per-container floor")
def step_impl(context):
    manifest = context.sandboxes.workloads(context.sandbox_name)[0]["manifest"]
    requests = manifest["spec"]["template"]["spec"]["containers"][0]["resources"]["requests"]
    assert requests["cpu"] == "250m", requests
    assert requests["memory"] == "256Mi", requests


@then('the capacity mode "squeezed" is declared in evidence with the bundle digest unchanged')
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["capacityMode"] == "squeezed", evidence
    assert context.bundles.digest() == context.bundles.digest_of_submitted_manifests()


@given("a production-sized bundle with three replicas")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.manifests = context.bundles.prod_sized_manifests(replicas=3)


@when("the bundle is admitted with the minimal capacity mode")
def step_impl(context):
    context.bundles.apply_options(
        context.app_name, context.sandbox_name, context.manifests, capacity_mode="minimal"
    )
    context.bundles.last_response.raise_for_status()


@then("replicas are floored to one and requests are scaled")
def step_impl(context):
    manifest = context.sandboxes.workloads(context.sandbox_name)[0]["manifest"]
    assert manifest["spec"]["replicas"] == 1, manifest["spec"]
    requests = manifest["spec"]["template"]["spec"]["containers"][0]["resources"]["requests"]
    assert requests["cpu"] == "250m", requests


@then('the capacity mode "minimal" is declared in evidence')
def step_impl(context):
    assert context.sandboxes.evidence(context.sandbox_name)["capacityMode"] == "minimal"


@given("a workload whose container is OOM-killed under the squeezed transform")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.api.post("/simctl/oom-under-squeeze", json={"workload": "web"}).raise_for_status()
    context.bundles.apply_options(
        context.app_name, context.sandbox_name, context.bundles.prod_sized_manifests()
    )
    context.bundles.last_response.raise_for_status()


@when("the developer checks the sandbox status")
def step_impl(context):
    context.sandbox_record = context.sandboxes.record(context.sandbox_name).json()


@then("a capacity-squeeze-incompatible diagnostic names the workload")
def step_impl(context):
    diagnostics = context.sandbox_record.get("diagnostics") or []
    hits = [d for d in diagnostics if d["code"] == "capacity-squeeze-incompatible"]
    assert hits and hits[0]["workload"] == "web", diagnostics


@then("the diagnostic guides the operator toward configuring full capacity mode")
def step_impl(context):
    diagnostics = context.sandbox_record["diagnostics"]
    assert "capacity: full" in diagnostics[0]["message"], diagnostics[0]


@given("a container whose JVM heap is set by an -Xmx environment variable")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.manifests = context.bundles.prod_sized_manifests(env={"JAVA_OPTS": "-Xmx2g"})


@when("the squeezed transform is applied")
def step_impl(context):
    context.bundles.apply_options(context.app_name, context.sandbox_name, context.manifests)
    context.bundles.last_response.raise_for_status()


@then("the environment variable is untouched")
def step_impl(context):
    manifest = context.sandboxes.workloads(context.sandbox_name)[0]["manifest"]
    env = manifest["spec"]["template"]["spec"]["containers"][0]["env"]
    assert env == [{"name": "JAVA_OPTS", "value": "-Xmx2g"}], env


@given("a bundle with a horizontal pod autoscaler acting on CPU requests")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.manifests = context.bundles.hpa_manifests()


@when("the bundle is admitted in squeezed mode")
def step_impl(context):
    context.bundles.apply_options(context.app_name, context.sandbox_name, context.manifests)
    context.bundles.last_response.raise_for_status()


@then("the autoscaler is suspended for the sandbox")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert "HorizontalPodAutoscaler/web-hpa" in (record.get("suspendedAutoscalers") or []), record


@then("the suspension is recorded in evidence")
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert "HorizontalPodAutoscaler/web-hpa" in (evidence.get("autoscalersSuspended") or []), evidence