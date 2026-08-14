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


def _ensure_app_and_sandbox(context):
    if not hasattr(context, "app_name"):
        _fresh_app_and_sandbox(context)


# --- B2: content-addressed bundles (task 2.5) ---


@given("two applies of byte-identical rendered manifests")
def step_impl(context):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.plain_mixed_manifests()


@when("both bundles are recorded")
def step_impl(context):
    first = context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)
    first.raise_for_status()
    context.first_digest = first.json()["digest"]
    second = context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)
    second.raise_for_status()
    context.second_digest = second.json()["digest"]


@then("both applies report the same bundle digest")
def step_impl(context):
    assert context.first_digest == context.second_digest, (
        "digests differ: %s vs %s" % (context.first_digest, context.second_digest)
    )


@given("a developer applies a manifest directory to their sandbox")
def step_impl(context):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.plain_mixed_manifests()
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)


@when("the apply completes")
def step_impl(context):
    context.bundles.last_response.raise_for_status()


@then("the control plane holds a bundle record addressed by its digest")
def step_impl(context):
    record = context.bundles.bundle_record(context.bundles.digest())
    assert record.ok, "no bundle record for digest %s" % context.bundles.digest()
    assert record.json()["digest"] == context.bundles.digest()


# --- B3: namespace transform (task 2.6) ---


@given('every manifest in the bundle declares metadata.namespace "team-a"')
def step_impl(context):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.namespaced_manifests("team-a")


@when("the bundle is admitted")
def step_impl(context):
    _ensure_app_and_sandbox(context)
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)


@then("the namespace is stripped by a namespace transform")
def step_impl(context):
    context.bundles.last_response.raise_for_status()
    kinds = [t["kind"] for t in context.bundles.transforms()]
    assert "namespace" in kinds, "expected a namespace transform, got %s" % kinds


@then("the transform is declared in the run's evidence")
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    kinds = [t["kind"] for t in evidence["transforms"]]
    assert "namespace" in kinds, "evidence lacks the namespace transform: %s" % evidence


@then("the bundle digest is unchanged by the transform")
def step_impl(context):
    assert context.bundles.digest() == context.bundles.digest_of_submitted_manifests(), (
        "digest was not computed over the submitted bytes"
    )


@given('a bundle whose manifests declare namespaces "team-a" and "team-b"')
def step_impl(context):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.namespaced_manifests("team-a", "team-b")


@then("the apply fails")
def step_impl(context):
    assert context.bundles.rejected(), (
        "expected the apply to fail, got %s" % context.bundles.last_response.status_code
    )


@then("the failure names the violating manifests and points to the multi-namespace path")
def step_impl(context):
    message = context.bundles.error_message()
    findings = context.bundles.findings()
    multi = [f for f in findings if f["code"] == "multi-namespace"]
    assert multi, "expected a multi-namespace finding, got %s" % findings
    assert "team-a" in message and "team-b" in message, message
    assert multi[0]["manifest"], "finding must name the violating manifests"
    assert "virtual-cluster" in message or "one application per namespace" in message, message


# --- B3: cluster-scoped rejection (task 2.7) ---


@given("a bundle containing a ClusterRole manifest")
def step_impl(context):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.cluster_scoped_manifests()


@then("the failure names the ClusterRole manifest and states that cluster-scoped resources belong to the substrate")
def step_impl(context):
    findings = context.bundles.findings()
    scoped = [f for f in findings if f["code"] == "cluster-scoped-resource"]
    assert scoped, "expected a cluster-scoped finding, got %s" % findings
    assert "ClusterRole" in scoped[0]["manifest"], scoped[0]
    assert "substrate" in scoped[0]["message"], scoped[0]


# --- B3: cross-namespace lint (task 2.8) ---


@given('a manifest referencing "{url}"')
def step_impl(context, url):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.manifests_referencing(url)
    context.referenced_url = url


@then("intake reports the reference with a suggested same-namespace rewrite")
def step_impl(context):
    findings = context.bundles.findings()
    lints = [f for f in findings if f["code"] == "cross-namespace-reference"]
    assert lints, "expected a cross-namespace lint finding, got %s" % findings
    assert "auth-api" in lints[0]["suggestion"], lints[0]


@then("the apply is not blocked by the lint finding")
def step_impl(context):
    assert context.bundles.accepted(), (
        "lint findings must not block: got %s" % context.bundles.last_response.status_code
    )


# --- B4: short names and declared aliases (task 2.9) ---


@given('the application\'s boundary contract declares the alias hostname "billing.deps.internal"')
def step_impl(context):
    context.applications.create("billing")
    spec = context.applications.draft("web")
    context.applications.with_dependency(spec, "billing", alias="billing.deps.internal")
    context.applications.admit(spec).raise_for_status()
    context.app_name = "web"
    context.sandbox_name = context.sandboxes.create("web")


@given('a manifest references "{url}"')
def step_impl(context, url):
    context.manifests = context.bundles.manifests_referencing(url)
    context.referenced_url = url


@then("the reference passes intake analysis")
def step_impl(context):
    context.bundles.last_response.raise_for_status()
    findings = context.bundles.findings()
    refs = [f for f in findings if f["code"] == "cross-namespace-reference"]
    assert not refs, "reference should pass, got findings: %s" % refs


# --- B5: rendered Helm output (task 2.10) ---


@given("the rendered output of a Helm chart as plain multi-document YAML")
def step_impl(context):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.rendered_helm_output()


@when("the developer applies the rendered output")
def step_impl(context):
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)
    context.bundles.last_response.raise_for_status()


@then("the bundle records the rendered manifests as its content")
def step_impl(context):
    record = context.bundles.bundle_record(context.bundles.digest())
    record.raise_for_status()
    assert record.json()["manifests"] == context.manifests, (
        "bundle content must be exactly the rendered output as submitted"
    )


# --- G7 intake: determinism (task 2.11) ---


@given("a bundle whose rendered manifests embed a generation timestamp")
def step_impl(context):
    _fresh_app_and_sandbox(context)
    context.manifests = context.bundles.timestamped_manifests()


@then("intake fails with a determinism error naming the offending source")
def step_impl(context):
    assert context.bundles.rejected(), "expected intake to fail"
    findings = context.bundles.findings()
    det = [f for f in findings if f["code"] == "non-deterministic-render"]
    assert det, "expected a determinism finding, got %s" % findings
    assert det[0]["manifest"], "determinism error must name the offending source"
    assert "determinism" in det[0]["message"], det[0]
