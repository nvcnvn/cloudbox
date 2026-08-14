"""Steps for the substrate-parity capability."""

import copy

from behave import given, when, then

BASE_COMPONENTS = [
    {"name": "cert-manager", "version": "v1.14.2", "kind": "operator", "ownsCRDs": ["cert-manager.io"]},
    {"name": "kafka-operator", "version": "v2.0.0", "kind": "operator", "ownsCRDs": ["kafka.strimzi.io"]},
    {"name": "pod-security-defaults", "version": "v1", "kind": "admission"},
    {"name": "storage-classes", "version": "v1", "kind": "class", "classes": ["fast-ssd", "standard"]},
]

CERT_MANAGER_BUNDLE = (
    "apiVersion: cert-manager.io/v1\nkind: Certificate\nmetadata:\n  name: web-cert\n"
    "spec:\n  dnsNames:\n    - web.internal\n"
    "---\n"
    "apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: data\n"
    "spec:\n  storageClassName: fast-ssd\n  resources:\n    requests:\n      storage: 10Gi\n"
)

IAM_SECRET_BUNDLE = (
    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
    "spec:\n  template:\n    metadata:\n      annotations:\n"
    "        eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/web\n"
    "    spec:\n      containers:\n        - name: web\n          image: web:1.0\n"
    "          envFrom:\n            - secretRef:\n                name: payment-api-key\n"
)


def _upgraded(components, name, version):
    updated = copy.deepcopy(components)
    for comp in updated:
        if comp["name"] == name:
            comp["version"] = version
    return updated


def _app_with_bundle(context, app_name, manifests, contract=None):
    fields = {"contract": contract} if contract else {}
    context.applications.create(app_name, **fields)
    sandbox = context.sandboxes.create(app_name, arrange=False)
    context.bundles.apply(app_name, sandbox, manifests)
    context.bundles.last_response.raise_for_status()
    return sandbox


# --- P1: application-scoped lockfile (task 3.15) ---


@given("an application whose bundles instantiate the cert-manager CRDs and name one storage class")
def step_impl(context):
    context.platform.ensure_split_platform(BASE_COMPONENTS)
    context.app_name = "web"
    context.sandbox_name = _app_with_bundle(context, "web", CERT_MANAGER_BUNDLE)


@when("the control plane maintains its substrate lockfile")
def step_impl(context):
    context.lockfile = context.platform.lockfile(context.app_name)


@when("the substrate lockfile is maintained")
def step_impl(context):
    context.lockfile = context.platform.lockfile(context.app_name)


@then("the lockfile records the Kubernetes minor version, those CRDs and their operator releases, applicable admission configurations, and that storage class")
def step_impl(context):
    lf = context.lockfile
    assert lf["kubernetesMinor"] == "1.31", lf
    names = {c["name"]: c["version"] for c in lf["components"]}
    assert names.get("cert-manager") == "v1.14.2", names
    assert "pod-security-defaults" in names, names
    assert "kafka-operator" not in names, "unreferenced operators must stay out of scope: %s" % names
    assert lf["classes"] == ["fast-ssd"], lf["classes"]


@then("a digest over that set identifies the lockfile")
def step_impl(context):
    assert context.lockfile["digest"].startswith("sha256:"), context.lockfile


@given("an operator installed on the cluster that the application's bundles never reference")
def step_impl(context):
    context.platform.ensure_split_platform(BASE_COMPONENTS)
    context.app_name = "web"
    context.sandbox_name = _app_with_bundle(context, "web", CERT_MANAGER_BUNDLE)
    context.digest_before = context.platform.lockfile("web")["digest"]
    context.soak_before = context.sandboxes.evidence(context.sandbox_name)["observedHealthySeconds"]


@when("that operator is upgraded")
def step_impl(context):
    context.platform.set_components(
        "prod-cluster", _upgraded(BASE_COMPONENTS, "kafka-operator", "v3.0.0")
    )


@then("the application's substrate digest is unchanged")
def step_impl(context):
    assert context.platform.lockfile("web")["digest"] == context.digest_before


@then("the application's in-flight evidence and soak time remain valid")
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["valid"], evidence
    assert evidence["observedHealthySeconds"] >= context.soak_before


@given("an application whose workloads use an IAM role binding and named secrets")
def step_impl(context):
    context.platform.ensure_split_platform(BASE_COMPONENTS)
    contract = {
        "secretNames": ["payment-api-key"], "ingressHostnames": [],
        "egressAllowlist": [], "dependencies": [],
    }
    context.applications.create("web", contract=contract)
    context.applications.supply_secret_value("web", "sandbox", "payment-api-key")
    context.app_name = "web"
    context.sandbox_name = context.sandboxes.create("web", arrange=False)
    context.bundles.apply("web", context.sandbox_name, IAM_SECRET_BUNDLE)
    context.bundles.last_response.raise_for_status()


@then("the identity bindings and secret values are recorded as declared-not-verified")
def step_impl(context):
    declared = context.lockfile["declaredNotVerified"]
    assert any("role-arn" in d for d in declared), declared
    assert any("payment-api-key" in d for d in declared), declared
    assert all("declared-not-verified" in d for d in declared), declared


# --- P2: digest in evidence, invalid on mismatch, audited override (task 3.16) ---


@given("a sandbox whose substrate digest differs from production's")
def step_impl(context):
    context.platform.ensure_split_platform(BASE_COMPONENTS)
    context.app_name = "web"
    context.sandbox_name = _app_with_bundle(context, "web", CERT_MANAGER_BUNDLE)
    # Production moves after the sandbox was provisioned; the sandbox cluster
    # stays behind, so the digests now differ.
    context.platform.set_components(
        "prod-cluster", _upgraded(BASE_COMPONENTS, "cert-manager", "v1.15.0")
    )


@when("the run's evidence is evaluated")
def step_impl(context):
    context.evidence = context.sandboxes.evidence(context.sandbox_name)


@then("the evidence is marked invalid for substrate mismatch")
def step_impl(context):
    assert not context.evidence["valid"], context.evidence
    reasons = " ".join(context.evidence["invalidReasons"])
    assert "substrate mismatch" in reasons, reasons


@given("evidence marked invalid for substrate mismatch")
def step_impl(context):
    context.execute_steps(
        'Given a sandbox whose substrate digest differs from production\'s\n'
        'When the run\'s evidence is evaluated\n'
        'Then the evidence is marked invalid for substrate mismatch'
    )


@when("an admin overrides the invalidation")
def step_impl(context):
    context.api.post(
        "/v1/sandboxes/%s/evidence/override-substrate" % context.sandbox_name,
        json={"reason": "accepted: cert-manager v1.15 is backward compatible"},
        headers={"X-Cloudbox-User": "admin@example.com"},
    ).raise_for_status()


@then("the override and the admin are recorded in the audit log")
def step_impl(context):
    entries = context.platform.audit_entries()
    hits = [e for e in entries if e["action"] == "substrate-mismatch-override"]
    assert hits, entries
    assert hits[0]["actor"] == "admin@example.com", hits[0]
    assert hits[0]["subject"] == context.sandbox_name, hits[0]


# --- P3: provisioned from the lockfile (task 3.17) ---


@given("an application substrate lockfile pinning an operator release")
def step_impl(context):
    context.platform.ensure_ready_platform()
    context.platform.set_components("main", BASE_COMPONENTS)
    context.app_name = "web"
    _app_with_bundle(context, "web", CERT_MANAGER_BUNDLE)
    context.lockfile = context.platform.lockfile("web")
    assert any(c["name"] == "cert-manager" for c in context.lockfile["components"])


@when("a sandbox is created on the shared cluster")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name, arrange=False)
    context.bundles.apply(context.app_name, context.sandbox_name, CERT_MANAGER_BUNDLE)
    context.bundles.last_response.raise_for_status()


@then("the sandbox's substrate provides that operator release")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    crds = context.platform.installed_crds(record["cluster"])
    assert crds, "sandbox cluster must be a set-up cluster"
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["substrateDigest"], evidence


@then("the sandbox's substrate digest matches the lockfile")
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["substrateDigest"] == context.lockfile["digest"], (
        "sandbox %s vs lockfile %s" % (evidence["substrateDigest"], context.lockfile["digest"])
    )


# --- P4: drift invalidates only referencing applications (task 3.18) ---


@given("applications A and B where only A's bundles instantiate operator X's CRDs")
def step_impl(context):
    context.platform.ensure_split_platform(BASE_COMPONENTS)
    context.sandbox_a = _app_with_bundle(context, "app-a", CERT_MANAGER_BUNDLE)
    context.sandbox_b = _app_with_bundle(context, "app-b", context.bundles.plain_mixed_manifests())
    context.digest_a = context.platform.lockfile("app-a")["digest"]
    context.digest_b = context.platform.lockfile("app-b")["digest"]


@when("operator X is upgraded in production")
def step_impl(context):
    context.platform.set_components(
        "prod-cluster", _upgraded(BASE_COMPONENTS, "cert-manager", "v1.15.0")
    )


@then("application A's lockfile digest changes and its stale sandboxes stop producing valid evidence")
def step_impl(context):
    assert context.platform.lockfile("app-a")["digest"] != context.digest_a
    evidence = context.sandboxes.evidence(context.sandbox_a)
    assert not evidence["valid"], evidence


@then("application B's lockfile digest and evidence are unaffected")
def step_impl(context):
    assert context.platform.lockfile("app-b")["digest"] == context.digest_b
    evidence = context.sandboxes.evidence(context.sandbox_b)
    assert evidence["valid"], evidence