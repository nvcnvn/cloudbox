"""Page object for the acceptance runner itself.

conformance-ci's untagged scenarios assert on the runner's own selection
behaviour: what a default or a conformance invocation of run_acceptance.py
selects. All knowledge of the wrapper's command line, of behave's dry-run JSON
output, and of the extracted tree's tag layout lives here; step definitions
see intent-level methods only.

Dry-run is the introspection mechanism: behave hooks do not fire (no app boot,
no OPENSPEC_ACCEPTANCE tripwire, no recursion into this very suite) and no
steps execute, but tag selection is exactly the real run's. behave's JSON
formatter omits tag-excluded scenarios entirely, so "attempted" == "present in
the output".
"""

import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent.parent  # acceptance-tests/

CONFORMANCE_TAG = "conformance"
_TAG_LINE = re.compile(r"^@\S")
_SCENARIO_LINE = re.compile(r"^(Scenario(?: Outline)?):\s*(.+)$")


class RunSelection:
    """One dry-run invocation's outcome: which scenarios were attempted."""

    def __init__(self, exit_code, scenarios):
        self.exit_code = exit_code
        self._scenarios = scenarios  # [{name, tags, status}]

    def attempted(self):
        return {s["name"] for s in self._scenarios}

    def failures(self):
        return {s["name"] for s in self._scenarios if s["status"] in ("failed", "error")}


class RunnerPage:
    """Invokes run_acceptance.py in dry-run mode and reports its selection."""

    def dry_run_default(self):
        return self._dry_run([])

    def dry_run_conformance(self):
        return self._dry_run(["--conformance"])

    def conformance_tagged_scenarios(self):
        """Scenario names carrying @conformance, directly or via their
        Feature, in the freshly extracted tree."""
        names = set()
        for feature in sorted((HERE / ".extracted").rglob("*.feature")):
            pending, feature_tags = [], []
            for raw in feature.read_text().splitlines():
                line = raw.strip()
                if _TAG_LINE.match(line):
                    pending += line.split()
                elif line.startswith("Feature:"):
                    feature_tags, pending = pending, []
                else:
                    match = _SCENARIO_LINE.match(line)
                    if match:
                        if ("@" + CONFORMANCE_TAG) in pending + feature_tags:
                            names.add(match.group(2).strip())
                        pending = []
                    elif line and not line.startswith("#"):
                        # Tags bind only across blank lines and comments.
                        pending = []
        return names

    def _dry_run(self, mode_args):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "dry-run.json"
            proc = subprocess.run(
                [sys.executable, "run_acceptance.py"]
                + mode_args
                + ["--dry-run", "-f", "json", "-o", str(out)],
                cwd=HERE, capture_output=True, text=True, timeout=120,
            )
            scenarios = []
            if out.exists() and out.stat().st_size:
                for feature in json.loads(out.read_text()):
                    feature_tags = set(feature.get("tags", []))
                    for element in feature.get("elements", []):
                        if element.get("type") == "scenario":
                            scenarios.append({
                                "name": element["name"],
                                "tags": set(element.get("tags", [])) | feature_tags,
                                "status": element.get("status"),
                            })
        return RunSelection(proc.returncode, scenarios)
