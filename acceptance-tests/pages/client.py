"""The World's thin transport: one HTTP client for the control plane and one
subprocess wrapper for the CLI. Page objects use these; they hold no domain
knowledge themselves.
"""

import subprocess

import requests


class ApiClient:
    def __init__(self, base_url):
        self.base_url = base_url
        self._session = requests.Session()

    def get(self, path, **kwargs):
        return self._session.get(self.base_url + path, timeout=10, **kwargs)

    def post(self, path, json=None, **kwargs):
        return self._session.post(self.base_url + path, json=json, timeout=10, **kwargs)

    def delete(self, path, **kwargs):
        return self._session.delete(self.base_url + path, timeout=10, **kwargs)


class CliResult:
    def __init__(self, completed):
        self.exit_code = completed.returncode
        self.stdout = completed.stdout
        self.stderr = completed.stderr
        self.output = completed.stdout + completed.stderr


class Cli:
    """Runs the real cloudbox binary. `offline=True` strips the server address,
    proving a command needs no control plane (X3)."""

    def __init__(self, cli_path, base_url):
        self._cli = cli_path
        self._base_url = base_url

    def run(self, *args, offline=False):
        cmd = [self._cli]
        if not offline:
            cmd += ["--server", self._base_url]
        cmd += list(args)
        return CliResult(
            subprocess.run(cmd, capture_output=True, text=True, timeout=60)
        )
