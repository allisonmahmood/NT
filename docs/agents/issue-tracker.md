# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues in `allisonmahmood/NT`. Use the `gh` CLI for all operations.

## Conventions

- Create, read, comment on, label, and close issues with the corresponding `gh issue` commands.
- Infer the repository from the current clone.
- Read an issue's complete body, comments, and labels before acting.
- GitHub shares one number space across issues and pull requests; resolve ambiguous references before acting.

## Pull requests as a triage surface

**PRs as a request surface: no.**

External pull requests do not enter the triage queue automatically. An explicitly named pull request may still be inspected or triaged.

## Skill operations

- When a skill says “publish to the issue tracker,” create a GitHub issue.
- When a skill says “fetch the relevant ticket,” read the complete GitHub issue and its comments.
- Apply the configured labels from `docs/agents/triage-labels.md`.

## Blocking relationships

Use GitHub's native issue dependencies as the canonical, UI-visible representation.

Add a blocker through the issue-dependencies API using the blocking issue's numeric database ID, not its `#number` or GraphQL node ID. If native dependencies are unavailable, add a `Blocked by: #<number>` line to the blocked issue.

A ticket is ready when all blocking issues are closed.
