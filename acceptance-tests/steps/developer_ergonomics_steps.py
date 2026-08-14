"""Steps for the developer-ergonomics capability."""

from behave import given, when, then

DEV = "dev@example.com"
MINH = "minh@example.com"


def _running_sandbox(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(
        context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests()
    )
    context.bundles.last_response.raise_for_status()


# --- X1: sealed logs / exec / port-forward (task 4.1) ---


@given("a developer who owns a running sealed sandbox")
def step_impl(context):
    _running_sandbox(context)


@when("they run the logs command against a workload in it")
def step_impl(context):
    context.cli_result = context.cli.run(
        "logs", "-s", context.sandbox_name, "-w", "web", as_user=DEV
    )


@then("the workload's logs stream to them")
def step_impl(context):
    assert context.cli_result.exit_code == 0, context.cli_result.output
    assert "streaming logs" in context.cli_result.output, context.cli_result.output


@then("the access is audited")
def step_impl(context):
    entries = context.platform.audit_entries()
    hits = [e for e in entries if e["action"] in ("logs", "exec", "port-forward")
            and e["subject"] == context.sandbox_name]
    assert hits and hits[0]["actor"] == DEV, entries


@given("a developer who owns a sealed sandbox running a web service")
def step_impl(context):
    _running_sandbox(context)
    context.allowlist_before = context.applications.declared_contract(
        context.app_name
    ).get("egressAllowlist") or []


@when("they port-forward to the service and open it locally")
def step_impl(context):
    context.cli_result = context.cli.run(
        "port-forward", "-s", context.sandbox_name, "-w", "web", as_user=DEV
    )
    assert context.cli_result.exit_code == 0, context.cli_result.output


@then("the traffic traverses the control plane")
def step_impl(context):
    assert "via control-plane" in context.cli_result.output, context.cli_result.output


@then("no ingress exception is added to the seal")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["sealed"], "the seal must remain intact"
    allowlist_after = context.applications.declared_contract(
        context.app_name
    ).get("egressAllowlist") or []
    assert allowlist_after == context.allowlist_before, (
        "port-forward must not touch seal policy"
    )


@when("developer Minh attempts to exec into one of its workloads")
def step_impl(context):
    context.cli_denial = context.cli.run(
        "exec", "-s", context.sandbox_name, "-w", "web", "-c", "ls", as_user=MINH
    )


# --- X2: status --explain and the allowlist proposal loop (task 4.2) ---


@given("a sandbox whose workload was denied egress to two undeclared endpoints")
def step_impl(context):
    _running_sandbox(context)
    for destination in ("api.first-vendor.com", "api.second-vendor.com"):
        resp = context.api.post(
            "/simctl/sandboxes/%s/egress-attempts" % context.sandbox_name,
            json={"workload": "web", "destination": destination},
        )
        resp.raise_for_status()
        assert not resp.json()["allowed"]


@when("the developer runs status with the explain option")
def step_impl(context):
    context.cli_result = context.cli.run(
        "status", "-a", context.app_name, "-s", context.sandbox_name, "--explain",
        as_user=DEV,
    )
    assert context.cli_result.exit_code == 0, context.cli_result.output


@then("each blocked attempt is rendered with destination, timestamp, and attributed workload")
def step_impl(context):
    out = context.cli_result.output
    for destination in ("api.first-vendor.com", "api.second-vendor.com"):
        assert destination in out, out
    assert "BLOCKED" in out and "workload web" in out, out
    assert "at 20" in out, "timestamps must be rendered: %s" % out


@then("a ready-to-submit allowlist change proposal for those endpoints is emitted for admin review")
def step_impl(context):
    out = context.cli_result.output
    assert "proposed allowlist change" in out, out
    assert "+ api.first-vendor.com" in out and "+ api.second-vendor.com" in out, out
    assert "admin review" in out, out


@given("a developer applying an existing application to a sandbox for the first time")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)


@when("they apply with the record-egress option")
def step_impl(context):
    context.bundles.apply_options(
        context.app_name, context.sandbox_name,
        context.bundles.plain_mixed_manifests(), record_egress=True,
    )
    context.bundles.last_response.raise_for_status()
    # The existing application immediately tries its (undeclared) dependencies.
    for destination in ("api.legacy-vendor.com", "queue.legacy-vendor.com"):
        context.api.post(
            "/simctl/sandboxes/%s/egress-attempts" % context.sandbox_name,
            json={"workload": "web", "destination": destination},
        ).raise_for_status()


@then("observed egress attempts are gathered into the same allowlist proposal loop as they occur")
def step_impl(context):
    explain = context.api.get("/v1/sandboxes/%s/explain" % context.sandbox_name).json()
    proposal = explain.get("proposal") or {}
    fqdns = proposal.get("addFqdns") or []
    assert "api.legacy-vendor.com" in fqdns and "queue.legacy-vendor.com" in fqdns, explain


# --- §6.8: the evidence-check level needs no promotion verbs (task 4.3) ---


@given("an application at the evidence-check adoption level")
def step_impl(context):
    context.platform.ensure_ready_platform()
    context.app_name = context.applications.create(
        "web", level="L2", scmIntegration=True
    )["name"]


@when("the team works a pull request from open to merge")
def step_impl(context):
    context.verbs_used = []

    check_dir = context.check.write_directory({"app.yaml": context.bundles.plain_mixed_manifests()})
    context.check.run(check_dir)
    assert context.check.exit_code() == 0
    context.verbs_used.append("check")

    context.pr_sandbox = context.scm.open_pr(context.app_name, "7")
    context.scm.push(context.app_name, "7", context.bundles.plain_mixed_manifests())
    context.verbs_used.append("apply")

    status = context.cli.run(
        "status", "-a", context.app_name, "-s", context.pr_sandbox["name"], as_user=DEV
    )
    assert status.exit_code == 0, status.output
    context.verbs_used.append("status")

    context.scm.close_pr(context.app_name, "7", merged=True)


@then("only the check, apply, status, and test verbs and the SCM integration are involved")
def step_impl(context):
    assert set(context.verbs_used) <= {"check", "apply", "status", "test"}, context.verbs_used
    record = context.sandboxes.record(context.pr_sandbox["name"]).json()
    assert record["state"] == "destroyed", "the PR flow must complete end to end"


@then("no promotion, approval, or rejection verb is required")
def step_impl(context):
    actions = [e["action"] for e in context.platform.audit_entries()]
    assert not any("promot" in a or "approv" in a or "reject" in a for a in actions), actions