"""Steps for the boundary-contract capability."""

from behave import given, when, then


# --- C1: exactly four kinds (task 2.13) ---


@given("an application being configured")
def step_impl(context):
    context.applications.create("billing")
    context.app_name = context.applications.create("web")["name"]


@when("its boundary contract is declared")
def step_impl(context):
    contract = context.applications.full_contract()
    context.applications.declare_contract(context.app_name, contract).raise_for_status()


@then("it holds secret names, ingress hostnames, an egress allowlist, and internal application dependencies")
def step_impl(context):
    contract = context.applications.declared_contract(context.app_name)
    assert contract["secretNames"] == ["payment-api-key"]
    assert contract["ingressHostnames"] == ["shop.example.com"]
    assert contract["egressAllowlist"] == ["api.stripe.com"]
    assert contract["dependencies"][0]["app"] == "billing"


@then("no other kind of environment-variant value is accepted into the contract")
def step_impl(context):
    fifth_kind = context.applications.full_contract()
    fifth_kind["configOverrides"] = {"LOG_LEVEL": "debug"}
    response = context.applications.declare_contract(context.app_name, fifth_kind)
    assert response.status_code == 422, (
        "a fifth contract kind must be rejected, got %s" % response.status_code
    )


# --- C2: secrets declared and valued (task 2.14) ---


@given('a bundle whose Deployment mounts the secret "payment-api-key"')
def step_impl(context):
    if not hasattr(context, "app_name"):
        context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.manifests = context.bundles.secret_mounting_manifests("payment-api-key")


@given('"payment-api-key" is not declared in the boundary contract')
def step_impl(context):
    contract = context.applications.declared_contract(context.app_name)
    assert "payment-api-key" not in (contract.get("secretNames") or [])


@when("the bundle is applied")
def step_impl(context):
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)


@then("the apply fails naming the undeclared secret")
def step_impl(context):
    assert context.bundles.rejected(), "expected the apply to fail"
    findings = context.bundles.findings()
    hits = [f for f in findings if f["code"] == "undeclared-secret"]
    assert hits and "payment-api-key" in hits[0]["message"], findings


@given('the contract declares the secret "payment-api-key"')
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    contract = {
        "secretNames": ["payment-api-key"],
        "ingressHostnames": [],
        "egressAllowlist": [],
        "dependencies": [],
    }
    context.applications.declare_contract(context.app_name, contract).raise_for_status()
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.manifests = context.bundles.secret_mounting_manifests("payment-api-key")


@given("no value is supplied for the sandbox environment")
def step_impl(context):
    pass  # deliberately no supply_secret_value call


@then("the apply fails naming the secret missing a value for the target environment")
def step_impl(context):
    assert context.bundles.rejected(), "expected the apply to fail"
    findings = context.bundles.findings()
    hits = [f for f in findings if f["code"] == "secret-missing-value"]
    assert hits and "payment-api-key" in hits[0]["message"], findings
    assert "value" in hits[0]["message"] and "environment" in hits[0]["message"], hits[0]


# --- C3: no variance outside the contract (task 2.15) ---


@given("a team wanting a per-environment log level in their manifests")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.wanted_variance = {"configOverrides": {"LOG_LEVEL": "debug"}}


@when("they look for an overlay or templating mechanism in the product")
def step_impl(context):
    contract = context.applications.full_contract(dependency_app=None)
    contract["dependencies"] = []
    contract.update(context.wanted_variance)
    context.contract_response = context.applications.declare_contract(context.app_name, contract)
    context.cli_result = context.cli.run("overlay", "--set", "LOG_LEVEL=debug")


@then("none exists")
def step_impl(context):
    assert context.contract_response.status_code == 422, (
        "the contract must reject variance outside the four kinds"
    )
    assert context.cli_result.exit_code != 0, "no overlay verb may exist in the CLI"
    assert "unknown command" in context.cli_result.output


@then("the documented path is changing the product spec, not configuring an overlay")
def step_impl(context):
    message = context.contract_response.json()["error"]
    assert "spec" in message and "overlay" in message, message


@given("a bundle admitted to a sandbox with a namespace transform and a squeezed capacity transform")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.manifests = context.bundles.namespaced_manifests("team-a")
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)
    context.bundles.last_response.raise_for_status()


@when("the run's evidence is inspected")
def step_impl(context):
    context.evidence = context.sandboxes.evidence(context.sandbox_name)


@then("both transforms are recorded in evidence")
def step_impl(context):
    kinds = sorted(t["kind"] for t in context.evidence["transforms"])
    assert kinds == ["capacity", "namespace"], (
        "expected namespace + capacity transforms, got %s" % kinds
    )
    squeezed = [t for t in context.evidence["transforms"] if t["kind"] == "capacity"]
    assert "squeezed" in squeezed[0]["detail"], squeezed[0]


@then("the bundle bytes and digest are unchanged")
def step_impl(context):
    assert context.bundles.digest() == context.bundles.digest_of_submitted_manifests()
    record = context.bundles.bundle_record(context.bundles.digest())
    record.raise_for_status()
    assert record.json()["manifests"] == context.manifests