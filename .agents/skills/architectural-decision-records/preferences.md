# ADR Preferences

preferred-style: custom

Custom house format, established by `adr/0001`–`adr/0007`: title line
`# NNNN — <decision>`, then a bullet list of `Status` / `Date` / `Supersedes`,
then `## Context`, `## Decision`, `## Consequences`. Alternatives considered
are woven into Context or Decision prose rather than a separate section.
Superseded ADRs keep status lines untouched; supersession is recorded in the
new ADR's `Supersedes` field only (files are left frozen).
