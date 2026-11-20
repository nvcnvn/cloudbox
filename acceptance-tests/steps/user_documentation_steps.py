"""Steps for the user-documentation capability (ADR 0011).

Pages are named, never pathed; required facts live in pages/docs.py.
"""

from behave import given, when, then


# --- The documentation has one front door that reaches every page (task 1.4) ---


@given("the published documentation set")
def step_impl(context):
    context.doc_pages = context.docs.published_pages()
    assert context.doc_pages, "the documentation set is empty"


@when("the links are followed from the repository README")
def step_impl(context):
    context.reached = context.docs.reachable_from_readme()


@then("every page in the documentation set is reached")
def step_impl(context):
    unreached = [p.name for p in context.doc_pages if p not in context.reached]
    assert not unreached, "not reachable from the README: %s" % unreached


@when("every link between documentation pages is followed")
def step_impl(context):
    context.broken_links = context.docs.broken_links()


@then("each link resolves to an existing page and, where it names one, an existing heading")
def step_impl(context):
    assert not context.broken_links, "broken links:\n  " + "\n  ".join(context.broken_links)


# --- Product terms are defined in the concepts page (task 1.5) ---


@given("the product's business glossary")
def step_impl(context):
    context.glossary_terms = context.docs.business_terms()
    assert context.glossary_terms, "the business glossary lists no terms"


@when("the concepts page is reviewed")
def step_impl(context):
    assert context.docs.exists("concepts"), "there is no concepts page"
    context.undefined_terms = context.docs.undefined_terms()


@then("every glossary term has a definition there")
def step_impl(context):
    assert not context.undefined_terms, (
        "glossary terms with no definition on the concepts page: %s" % context.undefined_terms
    )


@given("the concepts page")
def step_impl(context):
    assert context.docs.exists("concepts"), "there is no concepts page"
    context.doc_under_review = "concepts"


@when("an operator reads its account of the adoption levels")
def step_impl(context):
    context.level_entries = context.docs.adoption_level_entries()


@then("each of the four adoption levels is described")
def step_impl(context):
    missing = [lvl for lvl, entry in context.level_entries.items() if not entry]
    assert not missing, "adoption levels with no entry: %s" % missing


@then("only the sandbox level is presented as available in this release")
def step_impl(context):
    from pages.docs import AVAILABLE, NOT_AVAILABLE

    entries = {lvl: " ".join(e.lower().split()) for lvl, e in context.level_entries.items()}
    sandbox = entries.pop("L1")
    assert "sandbox" in sandbox, "L1 is not described as the sandbox level"
    assert AVAILABLE in sandbox and NOT_AVAILABLE not in sandbox, (
        "L1 is not presented as available in this release"
    )
    wrong = [lvl for lvl, e in entries.items() if NOT_AVAILABLE not in e]
    assert not wrong, "levels not stated as unavailable in this release: %s" % wrong


# --- The limitations page states what this release does not do (task 1.6) ---


@given("the limitations page")
def step_impl(context):
    assert context.docs.exists("limitations"), "there is no limitations page"
    context.doc_under_review = "limitations"


@then("it states that the control plane does not authenticate the identity a request acts as")
def step_impl(context):
    assert context.docs.states(context.reviewed_text, "no authentication"), (
        "the page does not state that requester identity is unauthenticated"
    )


@then("it does not present the control plane as a trust boundary between users")
def step_impl(context):
    assert context.docs.states(context.reviewed_text, "not a trust boundary"), (
        "the page does not say the control plane is not a trust boundary"
    )
    assert not context.docs.states(context.reviewed_text, "trust boundary claimed"), (
        "the page presents the control plane as a trust boundary"
    )


@then("it states that evidence is lost when the local control plane stops")
def step_impl(context):
    assert context.docs.states(context.reviewed_text, "evidence lost on stop")


@then("it states that registered clusters and applications do not survive a control-plane restart")
def step_impl(context):
    assert context.docs.states(context.reviewed_text, "state lost on restart")


@then("evidence checks, promotion, and setting production state are named as refused in this release")
def step_impl(context):
    from pages.docs import REFUSED_OPERATIONS

    refused = context.docs.section(context.doc_under_review, "refuse")
    assert refused, "the page has no section on refused operations"
    body = refused.lower()
    missing = [op for op in REFUSED_OPERATIONS if op not in body]
    assert not missing, "refused operations not named: %s" % missing


# --- The cluster setup guide (task 2.1) ---


@given("the cluster setup guide")
def step_impl(context):
    assert context.docs.exists("cluster setup"), "there is no cluster setup guide"
    context.doc_under_review = "cluster setup"


@when("the bootstrap script it names is located in the repository")
def step_impl(context):
    from pages.docs import REPO

    scripts = context.docs.bootstrap_scripts("cluster setup")
    assert scripts, "the setup guide names no bootstrap script"
    context.bootstrap_paths = [(REPO / s.lstrip("./")).resolve() for s in scripts]


@then("the script exists")
def step_impl(context):
    missing = [str(p) for p in context.bootstrap_paths if not p.is_file()]
    assert not missing, "named but absent: %s" % missing


@then("it is not under the conformance test fixtures")
def step_impl(context):
    fixtures = [str(p) for p in context.bootstrap_paths if "hack/conformance" in str(p)]
    assert not fixtures, "the user is pointed at a test fixture: %s" % fixtures


@when("an operator reads how registration can fail")
def step_impl(context):
    context.refusal_section = context.docs.section("cluster setup", "refused")
    assert context.refusal_section, "the guide has no section on refused registration"


@then("it states that a cluster which cannot be shown to enforce NetworkPolicy is refused")
def step_impl(context):
    assert context.docs.states(context.refusal_section, "registration refusal"), context.refusal_section


@then("it names what the operator can change to have the cluster accepted")
def step_impl(context):
    assert context.docs.states(context.refusal_section, "registration remedy"), context.refusal_section


# --- The CLI reference matches the command line in both directions (task 4.2) ---


@given("the commands the command line lists in its usage")
def step_impl(context):
    context.listed_commands = context.command_line.listed_commands()
    assert context.listed_commands, "the command line lists no commands"


@when("the CLI reference is reviewed")
def step_impl(context):
    assert context.docs.exists("cli reference"), "there is no CLI reference"
    context.documented_commands = context.docs.documented_commands()


@then("each listed command has a reference entry")
def step_impl(context):
    missing = sorted(context.listed_commands - context.documented_commands)
    assert not missing, "commands with no reference entry: %s" % missing


@given("the commands documented in the CLI reference")
def step_impl(context):
    context.documented_commands = context.docs.documented_commands()
    assert context.documented_commands, "the CLI reference documents no command"


@when("each is requested for its help from the command line")
def step_impl(context):
    context.unrecognised = sorted(
        c for c in context.documented_commands if not context.command_line.recognises(c)
    )


@then("the command line recognises every documented command")
def step_impl(context):
    assert not context.unrecognised, "documented but unknown to the binary: %s" % context.unrecognised


@given("the exit statuses the CLI reference documents")
def step_impl(context):
    context.documented_exits = context.docs.documented_exit_statuses()
    for outcome in ("success", "refused", "unreachable"):
        assert outcome in context.documented_exits, context.documented_exits


@when("a successful command, a refused command, and a command against an unreachable control plane are run")
def step_impl(context):
    surface = context.cli_surface
    context.actual_exits = {
        "success": surface.successful_command().exit_code,
        "refused": surface.refused_command().exit_code,
        "unreachable": surface.unreachable("status", "-s", "anything").exit_code,
    }


@then("each exit status is the one the reference documents for that outcome")
def step_impl(context):
    for outcome, code in context.actual_exits.items():
        assert context.documented_exits[outcome] == code, (outcome, code, context.documented_exits)


# --- The agent guide walks the whole sealed loop (task 4.3) ---


@given("the agent guide")
def step_impl(context):
    assert context.docs.exists("agent guide"), "there is no agent guide"
    context.doc_under_review = "agent guide"


@when("its sections are reviewed")
def step_impl(context):
    context.stage_coverage = context.docs.stage_sections("agent guide")


@then("it covers declaring, creating, applying, testing, explaining, amending, reading evidence, and destroying")
def step_impl(context):
    missing = [s for s, covered in context.stage_coverage.items() if not covered]
    assert not missing, "loop stages with no section: %s" % missing


@given("the commands shown in the agent guide's examples")
def step_impl(context):
    context.example_commands = context.docs.example_commands("agent guide")
    assert context.example_commands, "the agent guide shows no command"


@when("each is checked against the command line")
def step_impl(context):
    from pages.docs import GLOBAL_OPTIONS

    problems = []
    for argv in context.example_commands:
        args = list(argv)
        while args and args[0] in GLOBAL_OPTIONS:
            args = args[2:]
        command, rest = args[0], args[1:]
        if not context.command_line.recognises(command):
            problems.append("unknown command %r in %s" % (command, argv))
            continue
        usage = context.command_line.usage_of(command)
        for opt in (a for a in rest if a.startswith("-")):
            if opt not in usage:
                problems.append("option %s is not one %r accepts (%s)" % (opt, command, argv))
    context.example_problems = problems


@then("the command line recognises each command and each of its options")
def step_impl(context):
    assert not context.example_problems, "\n".join(context.example_problems)


@when("its explain step is reviewed")
def step_impl(context):
    context.explain_section = context.docs.section("agent guide", "explain")
    assert context.explain_section, "the agent guide has no explain step"


@then("it shows a blocked attempt and the allowlist proposal in the machine-readable rendering")
def step_impl(context):
    import json

    examples = [json.loads(b) for b in context.docs.json_examples(context.explain_section)]
    assert any(e.get("blockedEgress") and e.get("proposal") for e in examples), examples
    for e in examples:
        for attempt in e.get("blockedEgress", []):
            assert {"destination", "at", "workload"} <= set(attempt), attempt


# --- The containment page agrees with the product's statement (task 3.1) ---


@given("the containment statement the control plane publishes")
def step_impl(context):
    resp = context.api.get("/v1/containment-statement")
    resp.raise_for_status()
    context.containment = resp.json()


@when("the containment page is reviewed")
def step_impl(context):
    assert context.docs.exists("containment"), "there is no containment page"
    items = (
        context.containment["blockedChannels"]
        + context.containment["residualChannels"]
        + context.containment["recordingLimits"]
    )
    context.absent_items = [i for i in items if not context.docs.contains_verbatim("containment", i)]


@then("every blocked channel, residual channel, and recording limit in the statement appears on the page")
def step_impl(context):
    assert not context.absent_items, "statement items missing from the page:\n  " + "\n  ".join(context.absent_items)


@given("the containment page")
def step_impl(context):
    assert context.docs.exists("containment"), "there is no containment page"
    context.doc_under_review = "containment"


@then("the cooperative guarantee is stated separately from the adversarial one")
def step_impl(context):
    cooperative = context.docs.section("containment", "cooperative")
    adversarial = context.docs.section("containment", "adversarial")
    assert cooperative and adversarial, "the page does not separate the two guarantees"
    assert cooperative != adversarial


@then('the word "unbypassable" appears nowhere on the page')
def step_impl(context):
    assert "unbypassable" not in context.reviewed_text.lower()


# --- Every outcome the release names has a troubleshooting entry (task 5.1) ---


@given("the troubleshooting guide")
def step_impl(context):
    assert context.docs.exists("troubleshooting"), "there is no troubleshooting guide"


@when("the entry for {outcome} is looked up")
def step_impl(context, outcome):
    context.outcome_entry = context.docs.outcome_entry(outcome)
    assert context.outcome_entry, "no troubleshooting entry for %s" % outcome


@then("the entry states what the outcome means")
def step_impl(context):
    assert context.docs.states(context.outcome_entry, "means"), context.outcome_entry


@then("the entry states what the operator can do about it")
def step_impl(context):
    assert context.docs.states(context.outcome_entry, "remedy"), context.outcome_entry


# --- Refusal entries are keyed by reason code (add-stable-refusal-reasons 3.2) ---


@given("the refusal-reason catalog the command line lists")
def step_impl(context):
    context.listed_reasons = context.command_line.listed_reasons()


@when("the entry for each listed reason is looked up by its code")
def step_impl(context):
    context.reason_entries = {code: context.docs.reason_entry(code) for code in context.listed_reasons}


@then("every listed reason has an entry")
def step_impl(context):
    missing = [code for code, entry in context.reason_entries.items() if not entry]
    assert not missing, "no troubleshooting entry for: %s" % missing


@then("each entry states what the outcome means")
def step_impl(context):
    lacking = [c for c, e in context.reason_entries.items() if not context.docs.states(e, "means")]
    assert not lacking, "entries that do not say what the outcome means: %s" % lacking


@then("each entry states what the operator can do about it")
def step_impl(context):
    lacking = [c for c, e in context.reason_entries.items() if not context.docs.states(e, "remedy")]
    assert not lacking, "entries that do not say what to do: %s" % lacking


@given("the reason codes the troubleshooting guide keys its entries on")
def step_impl(context):
    context.keyed_reasons = context.docs.keyed_reason_codes()
    assert context.keyed_reasons, "the troubleshooting guide keys no entry on a reason code"


@when("each is compared with the refusal-reason catalog the command line lists")
def step_impl(context):
    listed = context.command_line.listed_reasons()
    context.unlisted_reasons = [c for c in context.keyed_reasons if c not in listed]


@then("the command line lists every one of them")
def step_impl(context):
    assert not context.unlisted_reasons, "keyed on reasons the product does not list: %s" % context.unlisted_reasons
