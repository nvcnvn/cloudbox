"""Steps for the evidence capability."""

from behave import given, when, then

DEV = "dev@example.com"

IMAGE_V1 = (
    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
    "spec:\n  replicas: 2\n  template:\n    spec:\n      containers:\n"
    "        - name: web\n          image: web:1.0\n"
)
IMAGE_V2 = IMAGE_V1.replace("web:1.0", "web:2.0")


def _advance(context, seconds):
    context.api.post("/simctl/advance-time", json={"seconds": seconds}).raise_for_status()


def _witnessed_run(context, fidelity=None, tests=84, hours=2, with_dependency=False):
    """A sealed managed run: applied, optionally datastore-graded, witnessed,
    soaked."""
    fields = {"testSuite": {"name": "smoke", "tests": tests}}
    if with_dependency:
        context.applications.create("billing")
        fields["contract"] = {
            "secretNames": [], "ingressHostnames": [], "egressAllowlist": [],
            "dependencies": [{"app": "billing", "alias": "billing.deps.internal"}],
        }
    context.app_name = context.applications.create("web", **fields)["name"]
    context.api.post("/simctl/production/%s" % context.app_name, json={"manifests": IMAGE_V1}).raise_for_status()
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, IMAGE_V2)
    context.bundles.last_response.raise_for_status()
    if fidelity:
        context.api.post(
            "/v1/sandboxes/%s/datastores" % context.sandbox_name,
            json={"name": "postgres", "fidelity": fidelity},
        ).raise_for_status()
    if tests:
        context.api.post(
            "/v1/sandboxes/%s/test" % context.sandbox_name, json={"suite": "smoke"},
            headers={"X-Cloudbox-User": DEV},
        ).raise_for_status()
    if hours:
        _advance(context, hours * 3600)


# --- G2: normalized diff (task 5.1) ---


@given("a bundle identical to production except for one changed container image")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.api.post("/simctl/production/%s" % context.app_name, json={"manifests": IMAGE_V1}).raise_for_status()
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, IMAGE_V2)
    context.bundles.last_response.raise_for_status()


@when("the developer diffs the bundle against production")
def step_impl(context):
    resp = context.api.get(
        "/v1/applications/%s/diff/%s" % (context.app_name, context.bundles.digest())
    )
    resp.raise_for_status()
    context.diff = resp.json().get("diff") or []


@then("the diff shows only the image change")
def step_impl(context):
    assert len(context.diff) == 1, "expected exactly one diff line, got %s" % context.diff
    line = context.diff[0]
    assert "web:1.0" in str(line["from"]) and "web:2.0" in str(line["to"]), line


@then("defaulted fields, managed fields, and recorded intake transforms produce no diff lines")
def step_impl(context):
    noise = [l for l in context.diff if any(
        marker in l["path"] for marker in ("creationTimestamp", "managedFields", "status", "namespace")
    )]
    assert not noise, noise


# --- G3: the full machine-gathered record (task 5.2) ---


@given("a sealed sandbox run that stayed healthy for two hours with a passing witnessed test suite")
def step_impl(context):
    _witnessed_run(context, fidelity="schema-replay", with_dependency=True)


@when("the control plane assembles the run's evidence")
def step_impl(context):
    context.evidence = context.sandboxes.evidence(context.sandbox_name)


@then("the evidence carries the bundle digest, the source sandbox, and the normalized diff")
def step_impl(context):
    ev = context.evidence
    assert ev["bundleDigest"].startswith("sha256:"), ev
    assert ev["sandbox"] == context.sandbox_name, ev
    assert isinstance(ev["diff"], list) and ev["diff"], ev["diff"]


@then("it records seal status, egress violation count, and substrate digest match")
def step_impl(context):
    ev = context.evidence
    assert ev["sealStatus"] == "sealed" and ev["egressViolations"] == 0, ev
    assert ev["substrateMatch"] is True and ev["substrateDigest"], ev


@then("it records per-datastore fidelity, capacity mode, intake transforms, readiness, and observed healthy duration")
def step_impl(context):
    ev = context.evidence
    assert ev["fidelity"] == {"postgres": "schema-replay"}, ev["fidelity"]
    assert ev["capacityMode"] == "squeezed", ev
    assert any(t["kind"] == "capacity" for t in ev["transforms"]), ev["transforms"]
    assert "workloads ready" in ev["readiness"], ev
    assert ev["observedHealthySeconds"] >= 2 * 3600, ev


@then("it records witnessed activity with test results and the declared dependency status")
def step_impl(context):
    ev = context.evidence
    assert ev["witnessed"]["events"] == 84, ev["witnessed"]
    assert ev["witnessed"]["tests"][0]["passed"] == 84, ev["witnessed"]
    assert ev["dependencies"] == [{"app": "billing", "status": "stubbed"}], ev["dependencies"]


# --- G6: honest wording (task 5.3) ---


@given("evidence for a run at fidelity profile-synthetic, capacity squeezed, healthy for two hours, with eighty-four witnessed test events")
def step_impl(context):
    _witnessed_run(context, fidelity="profile-synthetic")
    context.evidence = context.sandboxes.evidence(context.sandbox_name)


@when("the evidence summary is rendered")
def step_impl(context):
    if not hasattr(context, "evidence"):
        context.evidence = context.sandboxes.evidence(context.sandbox_name)
    context.summary = context.evidence["summary"]


@then("it states the run was sealed with zero undeclared dependency attempts on a substrate matching production")
def step_impl(context):
    assert "ran sealed with zero undeclared dependency attempts on a substrate matching production" in context.summary, context.summary


@then("it names the fidelity level, capacity mode, healthy duration, and witnessed activity count")
def step_impl(context):
    s = context.summary
    assert "profile-synthetic" in s and "squeezed" in s, s
    assert "7200" in s and "84 test/traffic events witnessed" in s, s


@then("it labels identity authorization and secret values declared-not-verified")
def step_impl(context):
    assert "declared-not-verified" in context.summary, context.summary


@then("it nowhere claims the change is verified working")
def step_impl(context):
    assert "verified working" not in context.summary.lower(), context.summary


@given("a run that booted healthy but received no test runs or traffic")
def step_impl(context):
    _witnessed_run(context, tests=0, hours=1)


@then("the witnessed activity count is zero and the run is presented as idle, not exercised")
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["witnessed"]["events"] == 0, evidence["witnessed"]
    assert "idle" in evidence["summary"], evidence["summary"]


# --- G7: PR binding by digest (task 5.4) ---


def _merged_pr(context, merge_manifests):
    context.app_name = context.applications.create("web", scmIntegration=True)["name"]
    context.platform.ensure_ready_platform()
    context.pr = "42"
    context.scm.open_pr(context.app_name, context.pr)
    context.manifests_a = context.bundles.plain_mixed_manifests()
    sandbox = context.scm.push(context.app_name, context.pr, context.manifests_a)
    context.sandbox_name = sandbox["name"]
    _advance(context, 3600)
    context.soak_at_merge = context.sandboxes.evidence(context.sandbox_name)["observedHealthySeconds"]
    context.api.post(
        "/v1/scm/events",
        json={"type": "merged", "app": context.app_name, "pr": context.pr,
              "author": DEV, "manifests": merge_manifests},
    ).raise_for_status()
    resp = context.api.get(
        "/v1/applications/%s/prs/%s/merge-result" % (context.app_name, context.pr)
    )
    resp.raise_for_status()
    context.merge_result = resp.json()


@given("a pull request whose sandbox produced valid evidence for digest A")
def step_impl(context):
    context.merge_with = "same"


@when("the merge result re-renders to digest A")
def step_impl(context):
    _merged_pr(context, context.bundles.plain_mixed_manifests())


@then("the evidence transfers to the merged commit with its accumulated soak time")
def step_impl(context):
    assert context.merge_result["status"] == "transferred", context.merge_result
    transferred = context.merge_result["evidence"]
    assert transferred["observedHealthySeconds"] >= context.soak_at_merge, transferred


@given("a pull request whose sandbox produced evidence for digest A")
def step_impl(context):
    context.merge_with = "different"


@when("the merge result re-renders to a different digest B")
def step_impl(context):
    diverged = context.bundles.plain_mixed_manifests() + "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: merge-extra\n"
    _merged_pr(context, diverged)


@then("the evidence is marked stale")
def step_impl(context):
    assert context.merge_result["status"] == "stale", context.merge_result


@then("the evidence check fails and any promotion blocks until a merged-tree run produces fresh evidence")
def step_impl(context):
    checks = context.api.get(
        "/v1/scm/prs/%s/%s/checks" % (context.app_name, context.pr)
    ).json()["checks"]
    failing = [c for c in checks if c["name"] == "cloudbox/evidence" and c["status"] == "fail"]
    assert failing, checks
    assert "fresh evidence" in context.merge_result["reason"], context.merge_result


# --- X4: witnessed activity attribution (task 5.5) ---


@given("an application with a declared test suite")
def step_impl(context):
    context.app_name = context.applications.create(
        "web", testSuite={"name": "smoke", "tests": 12}
    )["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()


@when("the developer runs the test command against their sandbox")
def step_impl(context):
    resp = context.api.post(
        "/v1/sandboxes/%s/test" % context.sandbox_name, json={"suite": "smoke"},
        headers={"X-Cloudbox-User": DEV},
    )
    resp.raise_for_status()
    context.test_run = resp.json()


@then("the suite executes as a Job inside the sealed sandbox")
def step_impl(context):
    workloads = context.sandboxes.workloads(context.sandbox_name)
    jobs = [w for w in workloads if w["name"] == "test-smoke"]
    assert jobs, workloads


@then("the control plane attributes the run and signs its results into the evidence as witnessed activity")
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["witnessed"]["tests"][0]["suite"] == "smoke", evidence["witnessed"]
    assert evidence["witnessed"]["events"] == 12, evidence["witnessed"]
    assert evidence["signature"].startswith("signed:controller:"), evidence


@given("a CI pipeline that triggers the test command")
def step_impl(context):
    context.app_name = context.applications.create(
        "web", testSuite={"name": "smoke", "tests": 12}
    )["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()
    context.api.post(
        "/v1/sandboxes/%s/test" % context.sandbox_name, json={"suite": "smoke"},
        headers={"X-Cloudbox-User": DEV},
    ).raise_for_status()


@when("the pipeline reports its own test outcome to the control plane")
def step_impl(context):
    context.assert_attempt = context.api.post(
        "/v1/sandboxes/%s/test-results" % context.sandbox_name,
        json={"suite": "pipeline-suite", "passed": 999, "total": 999},
        headers={"X-Cloudbox-User": "ci-pipeline"},
    )


@then("the reported outcome is not accepted as witnessed activity")
def step_impl(context):
    assert context.assert_attempt.status_code == 403, context.assert_attempt.text
    assert "witnessed activity" in context.assert_attempt.json()["error"]


@then("only the control-plane-attributed run appears in evidence")
def step_impl(context):
    witnessed = context.sandboxes.evidence(context.sandbox_name)["witnessed"]
    suites = [t["suite"] for t in witnessed["tests"]]
    assert suites == ["smoke"], witnessed


# --- G13: the signed evidence check (task 5.6) ---


@given("a managed-sandbox run with the seal held, zero egress violations, a substrate match, and fidelity and witnessed activity at policy minimums")
def step_impl(context):
    fields = {
        "testSuite": {"name": "smoke", "tests": 12},
        "policies": {"minFidelity": "schema-replay", "minWitnessedEvents": 1},
    }
    context.app_name = context.applications.create("web", **fields)["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()
    context.api.post(
        "/v1/sandboxes/%s/datastores" % context.sandbox_name,
        json={"name": "postgres", "fidelity": "schema-replay"},
    ).raise_for_status()
    context.api.post(
        "/v1/sandboxes/%s/test" % context.sandbox_name, json={"suite": "smoke"},
        headers={"X-Cloudbox-User": DEV},
    ).raise_for_status()


@when("the control plane evaluates the run for the pull request")
def step_impl(context):
    context.check_response = context.api.post(
        "/v1/evidence-checks", json={"sandbox": context.sandbox_name, "pr": "9"}
    )
    context.check_response.raise_for_status()
    context.posted_check = context.check_response.json()


@then("a signed passing status check is posted with the summary line and a link to the full evidence record")
def step_impl(context):
    check = context.posted_check
    assert check["status"] == "pass", check
    assert check["signature"].startswith("signed:controller:"), check
    assert "ran sealed" in check["summary"], check
    assert check["link"].endswith("/evidence"), check


@given("a managed-sandbox run that recorded one blocked egress attempt")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()
    context.api.post(
        "/simctl/sandboxes/%s/egress-attempts" % context.sandbox_name,
        json={"workload": "web", "destination": "api.undeclared-vendor.com"},
    ).raise_for_status()


@then("the posted status check fails and names the violation")
def step_impl(context):
    check = context.posted_check
    assert check["status"] == "fail", check
    assert "seal violations" in check["summary"] and "blocked egress" in check["summary"], check


@given("an application policy requiring fidelity of at least schema-replay")
def step_impl(context):
    context.app_name = context.applications.create(
        "web", policies={"minFidelity": "schema-replay"}
    )["name"]


@given("a run whose datastore fidelity was fixtures")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()
    context.api.post(
        "/v1/sandboxes/%s/datastores" % context.sandbox_name,
        json={"name": "postgres", "fidelity": "fixtures"},
    ).raise_for_status()


@then("the posted status check fails for fidelity below the policy minimum")
def step_impl(context):
    check = context.posted_check
    assert check["status"] == "fail", check
    assert "fidelity below the policy minimum" in check["summary"], check