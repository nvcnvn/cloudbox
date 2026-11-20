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

# --- The command line carries the whole loop (add-local-agent-mvp 3.3) ---

LOOP_STAGES = ("declare", "create", "apply", "test", "evidence", "destroy")


@given("a registered sandbox cluster and no source-control integration")
def step_impl(context):
    from pages.release import SOURCE_CONTROL

    context.platform.ensure_ready_platform()
    context.release.withdraw_integrations(SOURCE_CONTROL)


@when("an agent declares an application, creates a sandbox, applies a bundle, runs the declared suite, reads the evidence, and destroys the sandbox")
def step_impl(context):
    surface = context.cli_surface
    calls_before = context.api.calls
    runs = {}
    runs["declare"] = surface.declare_app("web", suite="smoke", tests=3)
    runs["create"] = surface.create_sandbox("web")
    sandbox = runs["create"].data()["name"]
    runs["apply"] = surface.apply("web", sandbox, context.bundles.plain_mixed_manifests())
    runs["test"] = surface.run_suite(sandbox)
    runs["evidence"] = surface.evidence(sandbox)
    runs["destroy"] = surface.destroy(sandbox)
    context.loop_runs = runs
    context.loop_api_calls = context.api.calls - calls_before


@then("every step is performed through the command line")
def step_impl(context):
    runs = context.loop_runs
    for stage in LOOP_STAGES:
        assert runs[stage].exit_code == 0, (stage, runs[stage].output)
    assert runs["apply"].data()["digest"].startswith("sha256:"), runs["apply"].stdout
    assert runs["test"].data()["total"] == 3, runs["test"].stdout
    evidence = runs["evidence"].data()
    assert evidence["sealStatus"] == "sealed" and evidence["signature"], evidence
    assert runs["destroy"].data()["destroyed"] is True


@then("no step requires addressing the control plane's interface directly")
def step_impl(context):
    assert context.loop_api_calls == 0, (
        "the loop addressed the control plane's interface %d time(s)" % context.loop_api_calls
    )


# --- Command output is available as data, with distinct exit statuses (3.4, 3.5) ---


@when("status with the explain option is run in machine-readable mode")
def step_impl(context):
    context.explain_run = context.cli_surface.explain(context.sandbox_name)
    assert context.explain_run.exit_code == 0, context.explain_run.output
    context.explain_data = context.explain_run.data()


@then("each blocked attempt is rendered as data with destination, timestamp, and attributed workload")
def step_impl(context):
    blocked = context.explain_data["blockedEgress"]
    assert {b["destination"] for b in blocked} == {"api.first-vendor.com", "api.second-vendor.com"}, blocked
    for attempt in blocked:
        assert attempt["at"] and attempt["workload"] == "web", attempt


@then("the allowlist change proposal is rendered as data alongside them")
def step_impl(context):
    proposal = context.explain_data["proposal"]
    assert sorted(proposal["addFqdns"]) == ["api.first-vendor.com", "api.second-vendor.com"], proposal
    assert proposal["app"] and proposal["submitTo"], proposal


@given("a sandbox with a recorded blocked egress attempt")
def step_impl(context):
    _running_sandbox(context)
    resp = context.api.post(
        "/simctl/sandboxes/%s/egress-attempts" % context.sandbox_name,
        json={"workload": "web", "destination": "api.first-vendor.com"},
    )
    resp.raise_for_status()


@when("status with the explain option is run in both renderings")
def step_impl(context):
    context.explain_data = context.cli_surface.explain(context.sandbox_name).data()
    human = context.cli_surface.explain(context.sandbox_name, json_output=False)
    assert human.exit_code == 0, human.output
    context.explain_human = human.stdout


def _facts(value):
    """Every scalar fact in a rendering, as text."""
    if isinstance(value, dict):
        for v in value.values():
            yield from _facts(v)
    elif isinstance(value, list):
        for v in value:
            yield from _facts(v)
    elif value is not None:
        yield str(value).lower() if isinstance(value, bool) else str(value)


@then("the machine-readable rendering carries the same facts as the human rendering")
def step_impl(context):
    human = context.explain_human.lower()
    missing = [fact for fact in _facts(context.explain_data) if fact.lower() not in human]
    assert not missing, (
        "the machine-readable rendering carries facts the human one does not: %s" % missing
    )


@given("a command that the control plane refuses and a command issued against an unreachable control plane")
def step_impl(context):
    context.refused_run_args = "refused"


@when("each is run")
def step_impl(context):
    surface = context.cli_surface
    context.outcome_runs = {
        "success": surface.successful_command(),
        "refused": surface.refused_command(),
        "unreachable": surface.unreachable("status", "-s", "anything"),
    }


@then("their exit statuses differ from each other and from a successful run")
def step_impl(context):
    from pages.cli import EXIT_OK

    codes = {k: r.exit_code for k, r in context.outcome_runs.items()}
    assert codes["success"] == EXIT_OK, codes
    assert len(set(codes.values())) == 3, codes
    for outcome in ("refused", "unreachable"):
        assert context.outcome_runs[outcome].data()["outcome"] == outcome, context.outcome_runs[outcome].stdout


# --- The command line lists every command it accepts (add-user-documentation 4.1) ---


@given("the command line")
def step_impl(context):
    pass


@when("its usage is requested")
def step_impl(context):
    context.listed_commands = context.command_line.listed_commands()


@then("every command the command line recognises is listed")
def step_impl(context):
    from pages.docs import SPEC_COMMANDS

    assert context.listed_commands == SPEC_COMMANDS, (
        "listed %s, specified %s" % (sorted(context.listed_commands), sorted(SPEC_COMMANDS))
    )


@when("each is requested for its help")
def step_impl(context):
    context.rejected = sorted(c for c in context.listed_commands if not context.command_line.recognises(c))


@then("none is rejected as an unknown command")
def step_impl(context):
    assert not context.rejected, context.rejected
