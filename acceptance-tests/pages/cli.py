"""The command line as the whole product surface (developer-ergonomics,
control-plane; add-local-agent-mvp section 3).

Two ways to reach a control plane:
- addressed: --server names the suite's control plane;
- local: no --server, so the CLI starts its own cloudboxd and reuses it. Each
  scenario gets a private state directory, and the started daemon runs the
  sim driver so the default suite needs no cluster.

Command strings and flags live here; steps name intents.
"""

import json
import os
import shutil
import signal
import subprocess
import tempfile
import time
from pathlib import Path

import requests

REPO = Path(__file__).resolve().parents[2]

EXIT_OK, EXIT_REFUSED, EXIT_USAGE, EXIT_UNREACHABLE = 0, 1, 2, 3
UNREACHABLE_SERVER = "http://127.0.0.1:1"


class CliRun:
    def __init__(self, completed, args):
        self.args = args
        self.exit_code = completed.returncode
        self.stdout = completed.stdout
        self.stderr = completed.stderr
        self.output = completed.stdout + completed.stderr

    def data(self):
        return json.loads(self.stdout)


class CliPage:
    def __init__(self, cli_path, base_url):
        self._cli = cli_path
        self._base_url = base_url
        self._state_dir = None
        self.runs = []

    # --- invocation --------------------------------------------------------

    def _run(self, args, server, as_user=None, json_output=True, env=None):
        cmd = [self._cli]
        if server:
            cmd += ["--server", server]
        if as_user:
            cmd += ["--as", as_user]
        if json_output:
            cmd += ["-o", "json"]
        cmd += [str(a) for a in args]
        run = CliRun(
            subprocess.run(cmd, capture_output=True, text=True, timeout=180, env=env),
            list(args),
        )
        self.runs.append(run)
        return run

    def addressed(self, *args, as_user="dev@example.com", json_output=True, server=None):
        return self._run(args, server or self._base_url, as_user, json_output)

    def local(self, *args, as_user="dev@example.com", json_output=True):
        env = dict(os.environ)
        env["CLOUDBOX_STATE_DIR"] = self.state_dir()
        env["CLOUDBOX_LOCAL_DRIVER"] = "sim"
        env["CLOUDBOXD"] = str(REPO / "bin" / "cloudboxd")
        return self._run(args, None, as_user, json_output, env)

    def unreachable(self, *args):
        return self._run(args, UNREACHABLE_SERVER)

    # --- the local control plane ---------------------------------------------

    def state_dir(self):
        if self._state_dir is None:
            self._state_dir = tempfile.mkdtemp(prefix="cloudbox-state-")
        return self._state_dir

    def local_state(self):
        path = Path(self.state_dir()) / "control-plane.json"
        if not path.exists():
            return None
        return json.loads(path.read_text())

    def local_running(self):
        st = self.local_state()
        if not st:
            return False
        try:
            os.kill(st["pid"], 0)
        except OSError:
            return False
        return True

    def arrange_local_host(self, name="dev-cluster"):
        """Sim arrangement on the CLI-started control plane: a cluster for the
        CLI to register (a real user brings one; see scripts/kind-cluster.sh)."""
        st = self.local_state()
        requests.post(st["addr"] + "/simctl/clusters", json={"name": name, "enforcing": True},
                      timeout=10).raise_for_status()
        return name

    def cleanup(self):
        st = self.local_state() if self._state_dir else None
        if st:
            try:
                os.kill(st["pid"], signal.SIGTERM)
                for _ in range(50):
                    os.kill(st["pid"], 0)
                    time.sleep(0.05)
            except OSError:
                pass
        if self._state_dir:
            shutil.rmtree(self._state_dir, ignore_errors=True)

    # --- the sealed loop -----------------------------------------------------

    def register_host(self, cluster, local=False):
        return (self.local if local else self.addressed)("cluster", "register", cluster)

    def declare_app(self, name, allow=(), suite=None, tests=0, local=False):
        args = ["app", "create", name]
        for fqdn in allow:
            args += ["--allow", fqdn]
        if suite:
            args += ["--suite", suite, "--tests", tests]
        return (self.local if local else self.addressed)(*args)

    def show_app(self, name, local=False):
        return (self.local if local else self.addressed)("app", "show", name)

    def amend_allowlist(self, app, *fqdns):
        return self.addressed("app", "allow", app, *fqdns)

    def create_sandbox(self, app, local=False):
        return (self.local if local else self.addressed)("sandbox", "create", "-a", app)

    def apply(self, app, sandbox, manifests, local=False):
        path = Path(self.state_dir()) / ("bundle-%d.yaml" % len(self.runs))
        path.write_text(manifests)
        return (self.local if local else self.addressed)("apply", "-a", app, "-s", sandbox, "-f", path)

    def run_suite(self, sandbox):
        return self.addressed("test", "-s", sandbox)

    def evidence(self, sandbox):
        return self.addressed("evidence", "-s", sandbox)

    def explain(self, sandbox, json_output=True):
        return self.addressed("status", "-s", sandbox, "--explain", json_output=json_output)

    def destroy(self, sandbox):
        return self.addressed("sandbox", "destroy", "-s", sandbox)

    def refused_command(self):
        return self.addressed("sandbox", "create", "-a", "no-such-application")

    def successful_command(self):
        return self.addressed("version")
