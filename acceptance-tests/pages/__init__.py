"""Page objects for the acceptance suite.

One page object per flow; all URLs and payload shapes live here, never in step
definitions. `attach(context)` is called from before_scenario so adding a page
object never requires touching environment.py.
"""

from pages.client import ApiClient, Cli
from pages.applications import ApplicationsPage
from pages.bundles import BundlesPage
from pages.check import CheckPage
from pages.platform import PlatformPage
from pages.runner import RunnerPage
from pages.sandboxes import SandboxesPage
from pages.scm import ScmPage


def attach(context):
    context.api = ApiClient(context.base_url)
    context.cli = Cli(context.cli_path, context.base_url)
    context.platform = PlatformPage(context.api)
    context.applications = ApplicationsPage(context.api)
    context.sandboxes = SandboxesPage(context.api, context.platform)
    context.bundles = BundlesPage(context.api)
    context.check = CheckPage(context.cli)
    context.scm = ScmPage(context.api)
    context.runner = RunnerPage()
