---
name: openspec-git-discipline
description: Use when running OpenSpec propose, continue, apply, verify, archive, or worktree workflows where proposal artifacts, branches, merges, or archive timing affect git history.
license: MIT
compatibility: Requires git and OpenSpec workflow artifacts.
---

# OpenSpec Git Discipline

## Core Rule

Every OpenSpec state change must cross `main` before the next lifecycle phase depends on it.

- Propose/continue artifacts may be drafted on a branch, but must be committed and merged to `main` before apply starts.
- Apply may run on `main`, a branch, or a worktree only if that exact proposal change is already available on `main`.
- Archive may run only from `main` after implementation is merged back.

Never create commits, branches, or merges unless the user explicitly asks.

## Gates

Two moments block. The rest are advisory: say the thing in the same response and keep
going. Never turn an advisory gate into a question that waits for an answer.

| Moment | Kind | Gate |
| --- | --- | --- |
| Before propose | Advisory | If not on `main`, say so in one line and continue. |
| During continue | Advisory | If the previous artifact is uncommitted, say so while creating the next one. |
| After propose | Advisory | State that proposal artifacts are uncommitted; offer a PR branch without waiting. |
| Before apply | **Blocking** | Stop unless the proposal change is committed on `main`. Then apply may run from `main`, a branch, or a worktree. |
| Before archive | **Blocking** | Stop unless implementation is merged back to `main` and archive is running from `main`. |
| After archive | Advisory | State that archive/spec sync changes are uncommitted. |

Only the two blocking gates protect correctness — applying or archiving against git
state that does not exist yet produces work that silently disappears. The advisory
moments protect tidiness, and a stop costs a full round trip, so they do not stop.

## Required Checks

Run these only at the two blocking gates. Advisory moments use git state you already
have — they never justify a fresh command.

Before apply:

1. Run `git status --short` and confirm `openspec/changes/<change>/` has no uncommitted
   proposal files.
2. Run `git ls-tree main --name-only -- openspec/changes/<change>`. Empty output means
   the proposal has not reached `main`. Do not improvise other commands for this.

Use this language if the proposal has not reached `main`:

> I should not apply this yet because the proposal change has not reached `main`. A proposal can be drafted on a branch, but apply must start only after that proposal state is available on `main`. Please merge or commit the proposal to `main` first, then I can apply from `main`, a branch, or a worktree.

Before archive:

1. Run `git branch --show-current` and `git status --short`.
2. Stop if not on `main`.
3. Stop if implementation work has not been merged back to `main`.

Use this language:

> I should not archive this yet because archive must run from `main` after implementation is merged back. Verify makes a change eligible to merge; it does not replace the merge.

## Red Flags

Stop, explain the boundary, and ask the user to make the git state explicit:

- Applying a proposal that exists only on the current branch/worktree.
- Treating worktree visibility as proof that the proposal reached `main`.
- Archiving from a feature branch or before implementation is merged to `main`.
- Auto-committing, branching, or merging without explicit user approval.

Say it and keep working — these are not stops:

- Uncommitted artifacts from a previous continue step.
- Proposing from a branch other than `main`.
- Uncommitted archive or spec sync output.
