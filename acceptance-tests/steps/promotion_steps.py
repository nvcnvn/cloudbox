"""Steps for the promotion capability."""

from behave import given, when, then

DEV = "priya@example.com"
APPROVER_ONE = "approver-one@example.com"
APPROVER_TWO = "approver-two@example.com"
SRE = "sre@example.com"
ENGINEER = "engineer@example.com"


def _promotion_app(context, name="web", level="L3", required=1, approvers=None, **extra):
    context.platform.ensure_ready_platform()
    fields = {
        "level": level,
        "approvers": approvers or [APPROVER_ONE, APPROVER_TWO],
        "policies": {"requiredApprovals": required},
    }
    if level == "L4":
        fields["breakGlassRole"] = [SRE]
    fields.update(extra)
    context.app_name = context.applications.create(name, **fields)["name"]
    return context.app_name


def _open_promotion(context, app=None, opened_by=DEV, manifests=None):
    app = app or context.app_name
    sandbox = context.sandboxes.create(app, owner=opened_by)
    context.sandbox_name = sandbox
    context.bundles.apply(app, sandbox, manifests or context.bundles.plain_mixed_manifests(), actor=opened_by)
    context.bundles.last_response.raise_for_status()
    resp = context.api.post(
        "/v1/promotions", json={"sandbox": sandbox}, headers={"X-Cloudbox-User": opened_by}
    )
    resp.raise_for_status()
    context.promotion = resp.json()
    return context.promotion


def _approve(context, actor, promotion_id=None, expect_ok=True):
    resp = context.api.post(
        "/v1/promotions/%s/approve" % (promotion_id or context.promotion["id"]),
        headers={"X-Cloudbox-User": actor},
    )
    if expect_ok:
        resp.raise_for_status()
        context.promotion = resp.json()
    return resp


def _promotion_state(context, promotion_id=None):
    resp = context.api.get("/v1/promotions/%s" % (promotion_id or context.promotion["id"]))
    resp.raise_for_status()
    return resp.json()


def _audit_actions(context, subject=None):
    entries = context.platform.audit_entries()
    if subject:
        entries = [e for e in entries if e["subject"] == subject]
    return [e["action"] for e in entries]


# --- G4: approvals and self-approval (task 6.1) ---


@given("an application policy requiring two approvers with the platform role")
def step_impl(context):
    _promotion_app(context, required=2)


@when("a promotion request is opened")
def step_impl(context):
    _open_promotion(context)


@then("the promotion remains pending until two platform-role approvals are recorded")
def step_impl(context):
    assert context.promotion["state"] == "pending", context.promotion
    _approve(context, APPROVER_ONE)
    assert _promotion_state(context)["state"] == "pending", "one approval must not suffice"
    _approve(context, APPROVER_TWO)
    assert _promotion_state(context)["state"] == "approved"


@given("a promotion request opened by developer Priya")
def step_impl(context):
    _promotion_app(context, approvers=[DEV, APPROVER_ONE])
    _open_promotion(context, opened_by=DEV)


@when("Priya attempts to approve it")
def step_impl(context):
    context.approve_attempt = _approve(context, DEV, expect_ok=False)


@then("the approval is rejected server-side as self-approval")
def step_impl(context):
    assert context.approve_attempt.status_code == 403, context.approve_attempt.text
    assert "self-approval" in context.approve_attempt.json()["error"]


# --- G5: synchronous audit (task 6.2) ---


@given("a promotion request moving through approval and apply")
def step_impl(context):
    _promotion_app(context)
    _open_promotion(context)


@when("each transition occurs")
def step_impl(context):
    _approve(context, APPROVER_ONE)
    context.api.post(
        "/v1/promotions/%s/apply" % context.promotion["id"],
        headers={"X-Cloudbox-User": APPROVER_ONE},
    ).raise_for_status()
    context.api.post("/simctl/gitops/%s/sync" % context.app_name).raise_for_status()


@then("a synchronous audit record is written before the transition completes")
def step_impl(context):
    actions = _audit_actions(context, subject=context.promotion["id"])
    for expected in ("promotion-created", "promotion-approved", "gitops-commit", "promotion-applied"):
        assert expected in actions, "missing %s in %s" % (expected, actions)
    assert _promotion_state(context)["state"] == "applied"


@given("the audit sink is unreachable")
def step_impl(context):
    _promotion_app(context)
    _open_promotion(context)
    _approve(context, APPROVER_ONE)
    context.api.post("/simctl/audit-sink", json={"available": False}).raise_for_status()


@when("an approved promotion attempts to apply")
def step_impl(context):
    context.apply_attempt = context.api.post(
        "/v1/promotions/%s/apply" % context.promotion["id"],
        headers={"X-Cloudbox-User": APPROVER_ONE},
    )


@then("the apply does not proceed until the audit record can be written")
def step_impl(context):
    assert context.apply_attempt.status_code == 503, context.apply_attempt.text
    assert "does not proceed" in context.apply_attempt.json()["error"]
    assert _promotion_state(context)["state"] == "approved", "state must not advance"
    # Sink restored: the same transition now proceeds.
    context.api.post("/simctl/audit-sink", json={"available": True}).raise_for_status()
    context.api.post(
        "/v1/promotions/%s/apply" % context.promotion["id"],
        headers={"X-Cloudbox-User": APPROVER_ONE},
    ).raise_for_status()
    assert _promotion_state(context)["state"] == "committed"


# --- G8 queued: merge opens a promotion (task 6.3) ---


@given("a pull request with a passing evidence check at a write-back-level application")
def step_impl(context):
    _promotion_app(context, scmIntegration=True)
    context.pr = "77"
    context.scm.open_pr(context.app_name, context.pr)
    context.manifests = context.bundles.plain_mixed_manifests()
    sandbox = context.scm.push(context.app_name, context.pr, context.manifests)
    context.sandbox_name = sandbox["name"]
    check = context.api.post(
        "/v1/evidence-checks", json={"sandbox": context.sandbox_name, "pr": context.pr}
    )
    check.raise_for_status()
    assert check.json()["status"] == "pass", check.json()


@when("the pull request is merged")
def step_impl(context):
    context.api.post(
        "/v1/scm/events",
        json={"type": "merged", "app": context.app_name, "pr": context.pr,
              "author": DEV, "manifests": context.manifests},
    ).raise_for_status()


@then("a promotion request carrying the transferred evidence is opened")
def step_impl(context):
    promotions = context.api.get("/v1/promotions?app=%s" % context.app_name).json()["promotions"]
    assert promotions, "merge must open a promotion at L3"
    context.promotion = promotions[0]
    evidence = context.promotion["evidence"]
    assert evidence and evidence["bundleDigest"] == context.promotion["digest"], context.promotion


@then("it awaits explicit approval under the application's policy")
def step_impl(context):
    assert context.promotion["state"] == "pending", context.promotion
    assert context.promotion["mode"] == "write-back", context.promotion


# --- G9: write-back (task 6.4) ---


@given("an approved promotion for bundle digest A at a write-back application")
def step_impl(context):
    _promotion_app(context)
    _open_promotion(context)
    _approve(context, APPROVER_ONE)
    context.digest_a = context.promotion["digest"]


@when("the controller commits the rendered bundle to the declared repository path")
def step_impl(context):
    context.api.post(
        "/v1/promotions/%s/apply" % context.promotion["id"],
        headers={"X-Cloudbox-User": APPROVER_ONE},
    ).raise_for_status()
    state = _promotion_state(context)
    assert state["state"] == "committed", (
        "after the commit, completion still waits on the team's GitOps apply: %s" % state
    )


@when("the team's GitOps controller applies the commit")
def step_impl(context):
    context.api.post("/simctl/gitops/%s/sync" % context.app_name).raise_for_status()


@then("the promotion completes only when the controller verifies live state matches digest A")
def step_impl(context):
    state = _promotion_state(context)
    assert state["state"] == "applied", state
    live = context.api.get("/v1/applications/%s/production-status" % context.app_name).json()
    assert live["liveDigest"] == context.digest_a, live


def _run_promotion_to_applied(context, app, level):
    _promotion_app(context, name=app, level=level)
    promo = _open_promotion(context, app=app)
    _approve(context, APPROVER_ONE, promotion_id=promo["id"])
    context.api.post(
        "/v1/promotions/%s/apply" % promo["id"], headers={"X-Cloudbox-User": APPROVER_ONE}
    ).raise_for_status()
    if level != "L4":
        context.api.post("/simctl/gitops/%s/sync" % app).raise_for_status()
    final = _promotion_state(context, promotion_id=promo["id"])
    assert final["state"] == "applied", final
    return final


@given("the same approved promotion executed once in write-back mode and once in direct mode")
def step_impl(context):
    context.writeback = _run_promotion_to_applied(context, "web-writeback", "L3")
    context.direct = _run_promotion_to_applied(context, "web-direct", "L4")


@when("the audit log and promotion evidence are compared")
def step_impl(context):
    canonical = ("promotion-created", "promotion-approval-recorded", "promotion-approved", "promotion-applied")
    context.sequences = {}
    for name, promo in (("write-back", context.writeback), ("direct", context.direct)):
        actions = [a for a in _audit_actions(context, subject=promo["id"]) if a in canonical]
        context.sequences[name] = actions


@then("the recorded evidence and audit semantics are identical")
def step_impl(context):
    assert context.sequences["write-back"] == context.sequences["direct"], context.sequences
    assert context.writeback["evidence"]["bundleDigest"] == context.direct["evidence"]["bundleDigest"]
    assert context.writeback["evidence"]["valid"] and context.direct["evidence"]["valid"]


# --- G11: rollback is a promotion (task 6.5) ---


@given("a previously applied production bundle with two weeks of observed-healthy history")
def step_impl(context):
    context.applied = _run_promotion_to_applied(context, "web", "L3")
    context.api.post("/simctl/advance-time", json={"seconds": 14 * 24 * 3600}).raise_for_status()


@when("an operator opens a rollback promotion for that digest in one command")
def step_impl(context):
    resp = context.api.post(
        "/v1/promotions/rollback",
        json={"app": context.app_name, "digest": context.applied["digest"]},
        headers={"X-Cloudbox-User": APPROVER_ONE},
    )
    resp.raise_for_status()
    context.rollback = resp.json()


@then("the promotion carries the bundle's original evidence and its production history")
def step_impl(context):
    rb = context.rollback
    assert rb["evidence"]["bundleDigest"] == context.applied["digest"], rb
    assert rb["history"]["observedHealthyLiveSeconds"] >= 14 * 24 * 3600, rb["history"]


@given("an approved promotion whose apply partially succeeds")
def step_impl(context):
    context.first_applied = _run_promotion_to_applied(context, "web", "L3")
    second = _open_promotion(
        context,
        manifests=context.bundles.plain_mixed_manifests() + "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: v2-extra\n",
    )
    _approve(context, APPROVER_ONE, promotion_id=second["id"])
    context.api.post(
        "/v1/promotions/%s/apply" % second["id"], headers={"X-Cloudbox-User": APPROVER_ONE}
    ).raise_for_status()
    context.api.post("/simctl/gitops/%s/fail-next-sync" % context.app_name).raise_for_status()


@when("the controller detects the live-state divergence")
def step_impl(context):
    context.api.post("/simctl/gitops/%s/sync" % context.app_name).raise_for_status()


@then("the promotion is left in the failed state with the divergence recorded")
def step_impl(context):
    state = _promotion_state(context)
    assert state["state"] == "failed", state
    assert "live state is" in state["divergence"], state


@then("the rollback path remains available")
def step_impl(context):
    resp = context.api.post(
        "/v1/promotions/rollback",
        json={"app": context.app_name, "digest": context.first_applied["digest"]},
        headers={"X-Cloudbox-User": APPROVER_ONE},
    )
    assert resp.status_code == 201, resp.text


# --- G1: strict mode single writer (task 7.1) ---


@given("an application at strict mode with a managed production namespace")
def step_impl(context):
    _promotion_app(context, level="L4")


@when("an engineer attempts to apply a manifest directly to that namespace")
def step_impl(context):
    context.write_attempt = context.api.post(
        "/simctl/production/%s" % context.app_name,
        json={"manifests": context.bundles.plain_mixed_manifests()},
        headers={"X-Cloudbox-User": ENGINEER},
    )


@then("the write is denied by the product-managed RBAC")
def step_impl(context):
    assert context.write_attempt.status_code == 403, context.write_attempt.text
    assert "product-managed RBAC" in context.write_attempt.json()["error"]


@given("the full v1 command surface")
def step_impl(context):
    context.usage = context.cli.run(offline=True).output


@when("a user searches for a flag or verb that applies a bundle straight to production")
def step_impl(context):
    context.env_prod_attempt = context.cli.run("apply", "--env", "prod", offline=True)
    context.promote_prod_attempt = context.cli.run("apply-to-prod", offline=True)


@then("no such flag or verb exists at any adoption level")
def step_impl(context):
    assert "--env" not in context.usage, context.usage
    assert context.env_prod_attempt.exit_code != 0
    assert "unknown command" in context.env_prod_attempt.output
    assert context.promote_prod_attempt.exit_code != 0
    assert "unknown command" in context.promote_prod_attempt.output


@given("an application at the evidence-check level")
def step_impl(context):
    context.platform.ensure_ready_platform()
    context.app_name = context.applications.create("web", level="L2")["name"]


@when("the control plane's credentials are inspected")
def step_impl(context):
    resp = context.api.get("/v1/applications/%s/credentials" % context.app_name)
    resp.raise_for_status()
    context.credentials = resp.json()


@then("the product holds no production write credentials")
def step_impl(context):
    assert context.credentials["productionWriteCredentials"] is False, context.credentials
    assert context.credentials["gitopsRepoWrite"] is False, context.credentials


@then("production is only observed and verified, never controlled")
def step_impl(context):
    assert "observed and verified" in context.credentials["posture"], context.credentials


# --- G12: break-glass (task 7.2) ---


@given("a strict-mode application with a configured break-glass role")
def step_impl(context):
    _promotion_app(context, level="L4")


@when("the emergency role requests break-glass access")
def step_impl(context):
    resp = context.api.post(
        "/v1/break-glass", json={"app": context.app_name}, headers={"X-Cloudbox-User": SRE}
    )
    resp.raise_for_status()
    context.grant = resp.json()


@then("auto-expiring write credentials are granted immediately without approval")
def step_impl(context):
    assert context.grant["expiresAt"], context.grant
    actions = _audit_actions(context, subject=context.app_name)
    assert "break-glass-granted" in actions, actions


@then("every action taken under them is audited")
def step_impl(context):
    context.api.post(
        "/simctl/production/%s" % context.app_name,
        json={"manifests": context.bundles.plain_mixed_manifests()},
        headers={"X-Cloudbox-User": SRE},
    ).raise_for_status()
    actions = _audit_actions(context, subject=context.app_name)
    assert "break-glass-write" in actions, actions


@given("a production write that did not come from an approved promotion")
def step_impl(context):
    context.applied = _run_promotion_to_applied(context, "web", "L4")
    context.api.post(
        "/v1/break-glass", json={"app": context.app_name}, headers={"X-Cloudbox-User": SRE}
    ).raise_for_status()
    context.api.post(
        "/simctl/production/%s" % context.app_name,
        json={"manifests": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: hotfix\n"},
        headers={"X-Cloudbox-User": SRE},
    ).raise_for_status()


@when("the controller detects the divergence")
def step_impl(context):
    context.production_status = context.api.get(
        "/v1/applications/%s/production-status" % context.app_name
    ).json()


@then("the divergence is recorded and the current bundle's evidence is invalidated")
def step_impl(context):
    promoted = context.production_status["promoted"]
    assert promoted["divergence"], promoted
    assert promoted["evidenceValid"] is False, promoted
    assert "divergence-detected" in _audit_actions(context, subject=context.app_name)


@then("validity returns only when a promotion adopting or reverting the divergence lands")
def step_impl(context):
    # Revert: a rollback promotion for the previously applied digest.
    resp = context.api.post(
        "/v1/promotions/rollback",
        json={"app": context.app_name, "digest": context.applied["digest"]},
        headers={"X-Cloudbox-User": DEV},
    )
    resp.raise_for_status()
    rollback = resp.json()
    _approve(context, APPROVER_ONE, promotion_id=rollback["id"])
    context.api.post(
        "/v1/promotions/%s/apply" % rollback["id"], headers={"X-Cloudbox-User": APPROVER_ONE}
    ).raise_for_status()
    promoted = context.api.get(
        "/v1/applications/%s/production-status" % context.app_name
    ).json()["promoted"]
    assert promoted["evidenceValid"] is True and not promoted.get("divergence"), promoted


@given("an application being configured for strict mode with no break-glass role")
def step_impl(context):
    context.platform.ensure_ready_platform()
    context.strict_spec = context.applications.draft("web", level="L4")


@when("setup runs")
def step_impl(context):
    context.applications.admit(context.strict_spec)


@then("setup fails stating that strict mode requires a configured break-glass role")
def step_impl(context):
    assert context.applications.rejected(), "L4 without break-glass must fail setup"
    message = context.applications.rejection_message()
    assert "break-glass" in message and "strict mode" in message, message