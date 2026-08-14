"""Steps for the data-fidelity capability."""

import json

from behave import given, when, then

DEV = "dev@example.com"
ADMIN = "admin@example.com"

SEED_VALUES = ["ada@example.com", "grace@example.com", "hopper-street-7"]

SEED_DB = {
    "schema": {"users": ["email", "address"]},
    "rows": [
        {"email": SEED_VALUES[0], "address": SEED_VALUES[2]},
        {"email": SEED_VALUES[1]},
    ],
}

MIGRATION_BUNDLE = (
    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
    "spec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: web:1.0\n"
    "---\n"
    "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: db-migrate-0042\n"
    "spec:\n  template:\n    spec:\n      containers:\n        - name: migrate\n          image: migrate:1.0\n"
)


def _datastore_app(context, name="web", datastores=None, **extra):
    context.platform.ensure_ready_platform()
    fields = {
        "datastores": datastores or [{"name": "postgres", "type": "postgresql"}],
    }
    fields.update(extra)
    context.app_name = context.applications.create(name, **fields)["name"]
    return context.app_name


def _profile(context, datastore="postgres"):
    resp = context.api.post(
        "/v1/applications/%s/datastores/%s/profile" % (context.app_name, datastore)
    )
    resp.raise_for_status()
    return resp.json()


def _provision(context, datastore, fidelity, sandbox=None):
    return context.api.post(
        "/v1/sandboxes/%s/datastores" % (sandbox or context.sandbox_name),
        json={"name": datastore, "fidelity": fidelity},
    )


# --- D1: profile lockfiles, values stay home (task 8.1) ---


@given("an application declaring a PostgreSQL datastore")
def step_impl(context):
    _datastore_app(context)
    context.api.post(
        "/simctl/databases/%s/postgres" % context.app_name, json=SEED_DB
    ).raise_for_status()


@when("the control plane profiles the production database")
def step_impl(context):
    context.profile = _profile(context)


@then("the profile records the schema digest and per-column statistics, content-addressed")
def step_impl(context):
    p = context.profile
    assert p["schemaDigest"].startswith("sha256:"), p
    assert p["digest"].startswith("sha256:"), p
    email = p["columns"]["email"]
    assert email["cardinality"] == 2 and email["nullRate"] == 0.0, email
    address = p["columns"]["address"]
    assert address["nullRate"] == 0.5, address
    assert p["rowCount"] == 2, p


@then("no production value leaves the production environment")
def step_impl(context):
    blob = json.dumps(context.profile)
    for value in SEED_VALUES:
        assert value not in blob, "production value %r escaped into the profile" % value


# --- D2: per-datastore declaration (task 8.2) ---


@given("a sandbox run against a schema-replay database and a fixtures cache")
def step_impl(context):
    _datastore_app(context, datastores=[
        {"name": "postgres", "type": "postgresql"},
        {"name": "cache", "type": "redis"},
    ])
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()
    _provision(context, "postgres", "schema-replay").raise_for_status()
    _provision(context, "cache", "fixtures").raise_for_status()


@when("the evidence is assembled")
def step_impl(context):
    context.evidence = context.sandboxes.evidence(context.sandbox_name)


@then("the database is declared at schema-replay and the cache at fixtures")
def step_impl(context):
    assert context.evidence["fidelity"] == {
        "postgres": "schema-replay", "cache": "fixtures"
    }, context.evidence["fidelity"]


# --- D3: policy minimums with conditional rules (task 8.3) ---


@given("an application policy requiring at least schema-replay for bundles containing a migration")
def step_impl(context):
    _datastore_app(context, policies={"minFidelityForMigrations": "schema-replay"})


@given("a bundle containing a migration run at fixtures fidelity")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, MIGRATION_BUNDLE)
    context.bundles.last_response.raise_for_status()
    _provision(context, "postgres", "fixtures").raise_for_status()


@when("the evidence is evaluated")
def step_impl(context):
    context.evidence = context.sandboxes.evidence(context.sandbox_name)


@then("the evidence is marked invalid for fidelity below the applicable minimum")
def step_impl(context):
    assert not context.evidence["valid"], context.evidence
    reasons = " ".join(context.evidence["invalidReasons"])
    assert "fidelity below the applicable minimum" in reasons, reasons


@then("the evidence check fails and any promotion blocks")
def step_impl(context):
    check = context.api.post(
        "/v1/evidence-checks", json={"sandbox": context.sandbox_name, "pr": "5"}
    )
    check.raise_for_status()
    assert check.json()["status"] == "fail", check.json()
    promotion = context.api.post(
        "/v1/promotions", json={"sandbox": context.sandbox_name},
        headers={"X-Cloudbox-User": DEV},
    )
    assert promotion.status_code == 409, promotion.text
    assert "evidence is not valid" in promotion.json()["error"]


# --- D4: migration replay (task 8.4) ---


@given("a datastore instantiated from the profile lockfile's schema")
def step_impl(context):
    _datastore_app(context)
    context.api.post(
        "/simctl/databases/%s/postgres" % context.app_name, json=SEED_DB
    ).raise_for_status()
    _profile(context)
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    _provision(context, "postgres", "schema-replay").raise_for_status()


@when("the bundle's migration chain runs and one migration fails")
def step_impl(context):
    context.api.post(
        "/simctl/fail-migration", json={"workload": "db-migrate-0042"}
    ).raise_for_status()
    context.bundles.apply(context.app_name, context.sandbox_name, MIGRATION_BUNDLE)
    context.bundles.last_response.raise_for_status()


@then("the failure appears in the sandbox status")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    failures = [d for d in record["diagnostics"] if d["code"] == "migration-failed"]
    assert failures and failures[0]["workload"] == "db-migrate-0042", record["diagnostics"]


@then("the failure is recorded in the run's evidence")
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    failures = evidence.get("migrationFailures") or []
    assert failures and "db-migrate-0042" in failures[0], evidence


# --- D5: profile drift (task 8.5) ---


@given("a sandbox provisioned from profile digest A")
def step_impl(context):
    _datastore_app(context)
    context.api.post(
        "/simctl/databases/%s/postgres" % context.app_name, json=SEED_DB
    ).raise_for_status()
    _profile(context)
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()
    _provision(context, "postgres", "schema-replay").raise_for_status()
    assert context.sandboxes.evidence(context.sandbox_name)["valid"]


@when("production's schema changes and the profile digest moves to B")
def step_impl(context):
    drifted = {
        "schema": {"users": ["email", "address", "phone_number"]},
        "rows": SEED_DB["rows"],
    }
    context.api.post(
        "/simctl/databases/%s/postgres" % context.app_name, json=drifted
    ).raise_for_status()
    _profile(context)  # the control plane re-profiles; the digest moves


@then("the sandbox's subsequent evidence is no longer valid at its declared fidelity level")
def step_impl(context):
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert not evidence["valid"], evidence
    reasons = " ".join(evidence["invalidReasons"])
    assert "data profile drift" in reasons and "schema-replay" in reasons, reasons


# --- D7: thin clones through the contract (task 8.6) ---


@given("an application using a database service with branching support")
def step_impl(context):
    _datastore_app(context, datastores=[
        {"name": "postgres", "type": "postgresql", "branching": True, "endpoint": "db.vendor.com"},
    ])
    context.api.put(
        "/v1/applications/%s/real-data" % context.app_name,
        json={"forAgents": False}, headers={"X-Cloudbox-User": ADMIN},
    ).raise_for_status()


@when("a sandbox is provisioned at live-clone fidelity")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    resp = _provision(context, "postgres", "live-clone")
    resp.raise_for_status()
    context.provision_result = resp.json()


@then("a per-sandbox branch endpoint is supplied as a per-sandbox secret with an allowlist entry")
def step_impl(context):
    result = context.provision_result
    assert result["branchEndpoint"].startswith("branch-%s" % context.sandbox_name), result
    assert context.sandbox_name in result["perSandboxSecret"], result
    attempt = context.api.post(
        "/simctl/sandboxes/%s/egress-attempts" % context.sandbox_name,
        json={"workload": "web", "destination": result["branchEndpoint"]},
    ).json()
    assert attempt["allowed"] and attempt["via"] == "egress-proxy", attempt


@then("the run's evidence declares that datastore at live-clone")
def step_impl(context):
    context.bundles.apply(context.app_name, context.sandbox_name, context.bundles.plain_mixed_manifests())
    context.bundles.last_response.raise_for_status()
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["fidelity"]["postgres"] == "live-clone", evidence["fidelity"]


# --- D8: admin-enabled, agent-gated (task 8.7) ---


@given("an application where no admin has enabled real-data levels")
def step_impl(context):
    _datastore_app(context)


@when("a developer requests a masked-snapshot sandbox")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name, owner=DEV)
    context.provision_attempt = _provision(context, "postgres", "masked-snapshot")


@then("the request is refused because real-data levels are not enabled for the application")
def step_impl(context):
    assert context.provision_attempt.status_code == 403, context.provision_attempt.text
    assert "not enabled" in context.provision_attempt.json()["error"]


@given("an application with live-clone enabled but no agent real-data policy")
def step_impl(context):
    _datastore_app(context, datastores=[
        {"name": "postgres", "type": "postgresql", "branching": True, "endpoint": "db.vendor.com"},
    ])
    context.api.put(
        "/v1/applications/%s/real-data" % context.app_name,
        json={"forAgents": False}, headers={"X-Cloudbox-User": ADMIN},
    ).raise_for_status()


@when("an agent-owned sandbox requests live-clone fidelity")
def step_impl(context):
    context.platform.ensure_ready_platform()
    resp = context.api.post(
        "/v1/sandboxes", json={"app": context.app_name, "agent": True},
        headers={"X-Cloudbox-User": "agent:codegen-7"},
    )
    resp.raise_for_status()
    context.sandbox_name = resp.json()["name"]
    context.provision_attempt = _provision(context, "postgres", "live-clone")


@then("the request is refused pending explicit policy for agent-owned sandboxes")
def step_impl(context):
    assert context.provision_attempt.status_code == 403, context.provision_attempt.text
    assert "agent-owned" in context.provision_attempt.json()["error"]