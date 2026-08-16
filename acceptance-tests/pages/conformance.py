"""Page object for the conformance subset definition.

Membership is the @conformance tag in the specs; the recorded exclusions live
in acceptance-tests/CONFORMANCE.md. All knowledge of that file's location and
table shape — and of what marks a scenario as soak-bound or as arranged
through a simulated external system — lives here, not in step definitions.
"""

from pages.runner import CONFORMANCE_TAG, HERE, extracted_scenarios

DEFINITION = HERE / "CONFORMANCE.md"

SOAK_MARKER = "soak"
# Phrases that mark an arrangement (Given/When) step as driving a simulated
# external system rather than the cluster.
EXTERNAL_SYSTEM_MARKERS = (
    "source control",
    "gitops",
    "audit sink",
    "audit log",
    "production",
    "seeded",
    "datastore",
    "pull request",
    "rebase",
)


class ConformanceSubsetPage:
    def subset(self):
        """The scenarios the conformance run selects, with their step text."""
        return [s for s in extracted_scenarios() if CONFORMANCE_TAG in s["tags"]]

    def soak_scenarios_in_subset(self):
        return [
            s["name"] for s in self.subset()
            if SOAK_MARKER in (s["name"] + " " + " ".join(s["steps"])).lower()
        ]

    def externally_arranged_in_subset(self):
        """Subset scenarios whose arrangement steps drive a simulated external
        system."""
        flagged = []
        for scenario in self.subset():
            arrangement = " ".join(
                step for step in scenario["steps"]
                if not step.startswith("Then")
            ).lower()
            if any(marker in arrangement for marker in EXTERNAL_SYSTEM_MARKERS):
                flagged.append(scenario["name"])
        return flagged

    def recorded_exclusions(self):
        """The exclusion records (excluded, reason) from the definition."""
        records = []
        for line in DEFINITION.read_text().splitlines():
            if not line.startswith("|") or set(line.strip()) <= set("|-: "):
                continue
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if len(cells) >= 2 and cells[0].lower() != "excluded":
                records.append({"excluded": cells[0], "reason": cells[1]})
        return records
