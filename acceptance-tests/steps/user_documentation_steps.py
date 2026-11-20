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
