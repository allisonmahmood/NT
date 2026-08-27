# Domain Docs

Engineering skills consume this repository's domain documentation before exploring the codebase.

## Before exploring

- Read `CONTEXT.md` at the repository root when it exists.
- Read relevant ADRs under `docs/adr/` when they exist.
- If these files do not exist, proceed silently. Domain-modeling skills create them lazily when terminology or decisions are resolved.

## Layout

This is a single-context repository:

- `CONTEXT.md` contains the project glossary and domain model.
- `docs/adr/` contains repository-wide architectural decisions.

## Consumer rules

Use glossary vocabulary in issue titles, implementation plans, tests, and refactor proposals. Do not introduce synonyms that the glossary rejects.

Surface conflicts with an existing ADR explicitly instead of silently overriding the recorded decision.
