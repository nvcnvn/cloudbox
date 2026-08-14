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


# --- CP3: shared vs split topologies (task 2.3) ---


@given("one application running sandboxes on the production cluster and another using a separate registered sandbox cluster")
def step_impl(context):
    from steps.substrate_parity_steps import BASE_COMPONENTS

    for name in ("prod-cluster", "sbx-cluster"):
        context.platform.ensure_cluster(name)
        context.platform.run_setup(name).raise_for_status()
        context.platform.set_components(name, BASE_COMPONENTS)
    context.platform.register_cluster("prod-cluster", "production")
    context.platform.register_cluster("sbx-cluster", "sandbox")
    context.applications.create("shared-app", sandboxCluster="prod-cluster")
    context.applications.create("split-app", sandboxCluster="sbx-cluster")


@when("both produce evidence for equivalent runs")
def step_impl(context):
    context.topology_evidence = {}
    for app in ("shared-app", "split-app"):
        sandbox = context.sandboxes.create(app, arrange=False)
        context.bundles.apply(app, sandbox, context.bundles.plain_mixed_manifests())
        context.bundles.last_response.raise_for_status()
        record = context.sandboxes.record(sandbox).json()
        context.topology_evidence[app] = (record, context.sandboxes.evidence(sandbox))
    shared_record, _ = context.topology_evidence["shared-app"]
    split_record, _ = context.topology_evidence["split-app"]
    assert shared_record["cluster"] == "prod-cluster" and split_record["cluster"] == "sbx-cluster"


@then("the evidence carries the same facts with the same validity rules in both topologies")
def step_impl(context):
    _, shared = context.topology_evidence["shared-app"]
    _, split = context.topology_evidence["split-app"]
    assert set(shared.keys()) == set(split.keys()), (
        "evidence fact sets differ between topologies"
    )
    for field in ("sealStatus", "valid", "substrateMatch", "capacityMode", "egressViolations"):
        assert shared[field] == split[field], (field, shared[field], split[field])
    assert shared["substrateDigest"] == split["substrateDigest"], (
        "same components must yield the same app-scoped substrate digest"
    )


# --- CP4: pipeline-forged checks impossible (task 5.7) ---


@given("a CI pipeline holding no control-plane signing capability")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.pr = "13"
    context.sandbox_name = context.sandboxes.create(context.app_name)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()


@when("the pipeline attempts to post an evidence status check to a pull request")
def step_impl(context):
    context.api.post(
        "/simctl/scm/prs/%s/%s/checks" % (context.app_name, context.pr),
        json={"name": "cloudbox/evidence", "summary": "forged: everything passed"},
    ).raise_for_status()


@then("the posted check is not the product's signed evidence check")
def step_impl(context):
    checks = context.api.get(
        "/v1/scm/prs/%s/%s/checks" % (context.app_name, context.pr)
    ).json()["checks"]
    forged = [c for c in checks if c["postedBy"] == "ci-pipeline"]
    assert forged, checks
    assert not forged[0].get("signature"), (
        "a pipeline post must carry no product signature: %s" % forged[0]
    )


@then("the product's check appears only when the controller posts it through the SCM integration")
def step_impl(context):
    checks = context.api.get(
        "/v1/scm/prs/%s/%s/checks" % (context.app_name, context.pr)
    ).json()["checks"]
    assert not [c for c in checks if c.get("signature")], "no signed check should exist yet"
    context.api.post(
        "/v1/evidence-checks", json={"sandbox": context.sandbox_name, "pr": context.pr}
    ).raise_for_status()
    checks = context.api.get(
        "/v1/scm/prs/%s/%s/checks" % (context.app_name, context.pr)
    ).json()["checks"]
    signed = [c for c in checks if c.get("signature", "").startswith("signed:controller:")]
    assert len(signed) == 1 and signed[0]["postedBy"] == "cloudbox-controller", checks
