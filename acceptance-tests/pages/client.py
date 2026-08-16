"""The World's thin transport: one HTTP client for the control plane and one
subprocess wrapper for the CLI. Page objects use these; they hold no domain
knowledge themselves.
"""

import subprocess

import requests


class ApiClient:
    """Default timeout suits the in-process sim; calls that drive a real
    cluster (sandbox creation runs a live enforcement probe) pass a larger
    one explicitly."""

    def __init__(self, base_url):
        self.base_url = base_url
        self._session = requests.Session()

    def get(self, path, **kwargs):
        kwargs.setdefault("timeout", 10)
        return self._session.get(self.base_url + path, **kwargs)

    def post(self, path, json=None, **kwargs):
        kwargs.setdefault("timeout", 10)
        return self._session.post(self.base_url + path, json=json, **kwargs)

    def put(self, path, json=None, **kwargs):
        kwargs.setdefault("timeout", 10)
        return self._session.put(self.base_url + path, json=json, **kwargs)

    def delete(self, path, **kwargs):
        kwargs.setdefault("timeout", 10)
        return self._session.delete(self.base_url + path, **kwargs)


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

    def run(self, *args, offline=False, as_user=None):
        cmd = [self._cli]
        if not offline:
            cmd += ["--server", self._base_url]
        if as_user:
            cmd += ["--as", as_user]
        cmd += list(args)
        return CliResult(
            subprocess.run(cmd, capture_output=True, text=True, timeout=60)
        )
