"""The published user documentation (user-documentation capability, ADR 0011).

All file paths, headings and required phrases live here; step definitions name
pages and facts, never paths or text. Checks assert coverage and required
facts, never exact wording (design D3): phrases match case-insensitively, and
entries are located by heading.
"""

import re
from collections import deque
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
DOCS = REPO / "docs"
README = REPO / "README.md"
BUSINESS_GLOSSARY = REPO / "glossary" / "business.md"

# Logical page name -> file. A page listed here that does not exist yet is
# simply not part of the published set until its task lands.
PAGES = {
    "index": DOCS / "README.md",
    "quickstart": DOCS / "quickstart.md",
    "concepts": DOCS / "concepts.md",
    "limitations": DOCS / "limitations.md",
    "cluster setup": DOCS / "cluster-setup.md",
    "cli reference": DOCS / "cli-reference.md",
    "agent guide": DOCS / "agent-guide.md",
    "containment": DOCS / "containment.md",
    "troubleshooting": DOCS / "troubleshooting.md",
}

# Required facts per page, as alternative phrasings (any one satisfies).
FACTS = {
    "no authentication": [
        "does not authenticate the identity a request acts as",
        "authenticates no one",
    ],
    "not a trust boundary": [
        "is not a trust boundary between users",
    ],
    "trust boundary claimed": [
        # Phrases that would present the control plane as a trust boundary.
        "is a trust boundary",
        "isolates users from each other",
    ],
    "enforcing cni": [
        "cni enforces networkpolicy",
        "cni that enforces networkpolicy",
        "networkpolicy-enforcing cni",
    ],
    "evidence lost on stop": [
        "evidence is lost when the local control plane stops",
    ],
    "state lost on restart": [
        "registered clusters and applications do not survive a control-plane restart",
    ],
}

REFUSED_OPERATIONS = ["evidence check", "promotion", "production state"]

ADOPTION_LEVELS = ["L1", "L2", "L3", "L4"]
AVAILABLE = "available in this release"
NOT_AVAILABLE = "not available in this release"

_LINK = re.compile(r"\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
_HEADING = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
_FENCE = re.compile(r"^(```|~~~)")


def slug(heading):
    """GitHub's heading anchor: lowercase, drop punctuation except hyphens and
    spaces, spaces become hyphens."""
    text = re.sub(r"[`*_]", "", heading).strip().lower()
    text = re.sub(r"[^\w\- ]", "", text)
    return text.replace(" ", "-")


def _lines_outside_fences(text):
    inside = False
    for line in text.splitlines():
        if _FENCE.match(line):
            inside = not inside
            continue
        if not inside:
            yield line


class DocsPage:
    # --- the published set -------------------------------------------------

    def published_pages(self):
        return sorted(p.resolve() for p in DOCS.rglob("*.md"))

    def path(self, name):
        return PAGES[name]

    def exists(self, name):
        return PAGES[name].is_file()

    def text(self, name):
        return PAGES[name].read_text()

    # --- links ---------------------------------------------------------------

    def links(self, path):
        """(target_file, anchor) for every relative link in a Markdown file.
        Absolute URLs are out of scope: they are not pages of this set."""
        out = []
        for line in _lines_outside_fences(path.read_text()):
            for target in _LINK.findall(line):
                if re.match(r"^[a-z][a-z0-9+.-]*:", target):
                    continue
                file_part, _, anchor = target.partition("#")
                resolved = (path.parent / file_part).resolve() if file_part else path.resolve()
                out.append((resolved, anchor))
        return out

    def reachable_from_readme(self):
        seen, queue = set(), deque([README.resolve()])
        while queue:
            current = queue.popleft()
            if current in seen or not current.is_file() or current.suffix != ".md":
                continue
            seen.add(current)
            for target, _ in self.links(current):
                queue.append(target)
        return seen

    def anchors(self, path):
        return {
            slug(m.group(2))
            for line in _lines_outside_fences(path.read_text())
            if (m := _HEADING.match(line))
        }

    def broken_links(self):
        broken = []
        for page in [README.resolve()] + self.published_pages():
            for target, anchor in self.links(page):
                if not target.exists():
                    broken.append("%s -> %s (no such file)" % (page.name, target))
                elif anchor and target.suffix == ".md" and anchor not in self.anchors(target):
                    broken.append("%s -> %s#%s (no such heading)" % (page.name, target.name, anchor))
        return broken

    # --- headings and sections -----------------------------------------------

    def headings(self, name):
        return [
            m.group(2)
            for line in _lines_outside_fences(self.text(name))
            if (m := _HEADING.match(line))
        ]

    def section(self, name, heading_phrase):
        """Text under the first heading containing heading_phrase, up to the
        next heading of the same or higher level. None if absent."""
        lines = self.text(name).splitlines()
        start = level = None
        for i, line in enumerate(lines):
            m = _HEADING.match(line)
            if not m:
                continue
            if start is None and heading_phrase.lower() in m.group(2).lower():
                start, level = i, len(m.group(1))
            elif start is not None and len(m.group(1)) <= level:
                return "\n".join(lines[start:i])
        return "\n".join(lines[start:]) if start is not None else None

    # --- facts ---------------------------------------------------------------

    def states(self, text, fact):
        body = " ".join(text.lower().split())
        return any(p.lower() in body for p in FACTS[fact])

    # --- glossary and concepts -----------------------------------------------

    def business_terms(self):
        terms = []
        for line in BUSINESS_GLOSSARY.read_text().splitlines():
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if len(cells) >= 2 and cells[0] and cells[0] != "Term" and not set(cells[0]) <= {"-", " "}:
                terms.append(cells[0])
        return terms

    def undefined_terms(self):
        """Business terms with no heading of their own on the concepts page."""
        headings = [h.lower() for h in self.headings("concepts")]
        return [t for t in self.business_terms() if not any(t.lower() == h for h in headings)]

    def adoption_level_entries(self):
        """Level -> the concepts-page section describing it (None if absent)."""
        return {lvl: self.section("concepts", lvl) for lvl in ADOPTION_LEVELS}

    # --- checked against the product (design D2) ---

    def bootstrap_scripts(self, name):
        return sorted(set(_BOOTSTRAP.findall(self.text(name))))

    def documented_commands(self):
        out = set()
        for h in self.headings("cli reference"):
            m = re.match(r"`cloudbox ([\w-]+)`", h.strip())
            if m:
                out.add(m.group(1))
        return out

    def documented_exit_statuses(self):
        """outcome -> exit status, from the reference's exit-status table."""
        table = self.section("cli reference", "exit statuses") or ""
        out = {}
        for line in table.splitlines():
            cells = [c.strip().strip("`") for c in line.strip().strip("|").split("|")]
            if len(cells) >= 2 and cells[0].isdigit():
                out[cells[1].lower()] = int(cells[0])
        return out

    def example_commands(self, name):
        """Each `cloudbox ...` line in the page's fenced sh blocks, as argv."""
        out = []
        for block in _fenced(self.text(name), "sh"):
            for line in block.splitlines():
                line = line.split("#", 1)[0].strip()
                if line.startswith("cloudbox "):
                    out.append(line.split()[1:])
        return out

    def json_examples(self, text):
        return _fenced(text, "json")

    def stage_sections(self, name):
        heads = [h.lower() for h in self.headings(name)]
        return {stage: any(stage in h for h in heads) for stage in LOOP_STAGES}

    def contains_verbatim(self, name, phrase):
        return _norm(phrase) in _norm(self.text(name))

    def outcome_entry(self, outcome):
        return _section_body(self.text("troubleshooting"), OUTCOMES[outcome])


# --- product-checked facts (design D2) -----------------------------------------

import subprocess as _subprocess

# The commands the specs require of the command line (design D4).
SPEC_COMMANDS = {
    "check", "cluster", "app", "sandbox", "apply", "test", "status", "evidence",
    "logs", "exec", "port-forward", "server", "version", "reasons",
}
GLOBAL_OPTIONS = {"--server", "--as", "-o", "--output"}
LOOP_STAGES = ["declare", "create", "apply", "test", "explain", "amend", "evidence", "destroy"]

# Troubleshooting outcome (spec Examples) -> its entry's heading phrase.
OUTCOMES = {
    "a cluster refused for unproven enforcement": "unproven network policy enforcement",
    "an operation refused for a missing integration": "no integration configured",
    "an egress proxy reported unavailable": "egress proxy unavailable",
    "substrate parity reported unverified": "substrate parity unverified",
    "an unreachable control plane": "control plane unreachable",
}

_UNREACHABLE = "http://127.0.0.1:1"
_BOOTSTRAP = re.compile(r"(?<![\w/.-])((?:\.?/)?scripts/[\w./-]+\.sh)")


def _norm(text):
    return " ".join(text.split()).lower()


class CommandLine:
    """The binary as the authority for what the CLI reference and agent guide
    may claim."""

    def __init__(self, cli_path):
        self._cli = cli_path

    def run(self, *args, server=_UNREACHABLE):
        cmd = [self._cli]
        if server:
            cmd += ["--server", server]
        return _subprocess.run(cmd + list(args), capture_output=True, text=True, timeout=60)

    def listed_commands(self):
        usage = _subprocess.run([self._cli], capture_output=True, text=True).stderr
        block = usage.split("Commands:", 1)[1] if "Commands:" in usage else ""
        names = set()
        for line in block.splitlines():
            if not line.startswith("  ") or not line.strip():
                if names:
                    break
                continue
            names.add(line.split()[0])
        return names

    def usage_of(self, command):
        """The command's own usage text (run without flags)."""
        out = self.run(command)
        return out.stdout + out.stderr

    def recognises(self, command):
        return "unknown command" not in self.usage_of(command)


def _section_body(text, heading_phrase):
    lines = text.splitlines()
    start = level = None
    for i, line in enumerate(lines):
        m = _HEADING.match(line)
        if not m:
            continue
        if start is None and heading_phrase.lower() in m.group(2).lower():
            start, level = i, len(m.group(1))
        elif start is not None and len(m.group(1)) <= level:
            return "\n".join(lines[start:i])
    return "\n".join(lines[start:]) if start is not None else None


def _fenced(text, lang):
    blocks, current, inside = [], [], False
    for line in text.splitlines():
        if line.startswith("```"):
            if inside:
                blocks.append("\n".join(current))
                current, inside = [], False
            elif line.strip() == "```" + lang:
                inside = True
            continue
        if inside:
            current.append(line)
    return blocks



FACTS.update({
    "registration refusal": ["unproven network policy enforcement"],
    "registration remedy": ["install a cni that enforces networkpolicy"],
    "means": ["**what it means.**"],
    "remedy": ["**what to do.**"],
})
