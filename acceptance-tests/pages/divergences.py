"""Page object for the sim divergence record (internal/sim/DIVERGENCES.md).

The conformance-ci reconciliation rule requires every sim-vs-real divergence
to be corrected in the simulation and recorded with the behaviour that
differed; this reads the record the rule points at.
"""

from pathlib import Path

HERE = Path(__file__).resolve().parent.parent  # acceptance-tests/
RECORD = HERE.parent / "internal" / "sim" / "DIVERGENCES.md"


class SimDivergencesPage:
    def records(self):
        if not RECORD.is_file():
            return []
        rows = []
        for line in RECORD.read_text().splitlines():
            if not line.startswith("|") or set(line.strip()) <= set("|-: #"):
                continue
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if len(cells) >= 3 and cells[0] != "#":
                rows.append({
                    "id": cells[0],
                    "behaviour": cells[1],
                    "correction": cells[2],
                })
        return rows
