"""Page objects for the acceptance suite.

One page object per flow; all URLs and payload shapes live here, never in step
definitions. `attach(context)` is called from before_scenario so adding a page
object never requires touching environment.py.
"""

from pages.client import ApiClient, Cli


def attach(context):
    context.api = ApiClient(context.base_url)
    context.cli = Cli(context.cli_path, context.base_url)
    # Flow page objects are attached here as their capabilities land.
