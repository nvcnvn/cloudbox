"""Offline-check page object: writes manifest fixtures to a directory and runs
the real `cloudbox check` binary with no server and no cluster (X3).
"""

import tempfile
from pathlib import Path


class CheckPage:
    def __init__(self, cli):
        self._cli = cli
        self.result = None

    def write_directory(self, manifests_by_file):
        directory = Path(tempfile.mkdtemp(prefix="cloudbox-check-"))
        for filename, content in manifests_by_file.items():
            (directory / filename).write_text(content)
        return str(directory)

    def run(self, directory):
        self.result = self._cli.run("check", "-f", directory, offline=True)
        return self.result

    def exit_code(self):
        return self.result.exit_code

    def reported_compatible(self):
        return "compatible" in self.result.output

    def findings_reported(self, *codes):
        return all(code in self.result.output for code in codes)

    def names_manifest_and_fix(self):
        out = self.result.output
        return ("/" in out) and ("fix" in out.lower() or "substrate" in out or "split" in out)
