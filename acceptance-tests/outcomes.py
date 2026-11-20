"""Outcomes several capabilities assert on with the same words.

Where two capabilities' scenarios share a Then — "the request is refused" is
the current case, asserted by sandbox-seal and by kube-driver — behave allows
exactly one definition of it. The shared step must therefore not know which
surface produced the outcome: each capability's When-step records what it
observed, and the Then asserts on that.

The criterion (an HTTP status, a marker in a pod's output) is knowledge of the
surface, so it lives with the step that used that surface, not in the shared
assertion.
"""


class Refusal:
    """Whether a request was refused, and what to print if it was not. A
    surface that reports the refusal's reason (and, for not-found, its
    subject) records those too (ADR 0012)."""

    def __init__(self, refused, detail, reason=None, subject=None):
        self.refused = bool(refused)
        self.detail = detail
        self.reason = reason
        self.subject = subject
