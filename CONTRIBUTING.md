# Contributing to VeriFHIR Gateway

Thank you for your interest in contributing.

## Development Setup

1. Install Go 1.22+
2. Clone the repository
3. Run the project:

```powershell
go run .\cmd\gateway\main.go
```

## Branching

- Use feature branches from `main`
- Branch naming:
  - `feat/<short-description>`
  - `fix/<short-description>`
  - `docs/<short-description>`

## Commit Style

Use clear and focused commits.

Examples:
- `feat(parser): add ADT A01 segment validation`
- `fix(mapping): handle missing PID-3`
- `docs(readme): add quickstart notes`

## Pull Request Guidelines

Before opening a PR:

1. Keep changes scoped to one concern
2. Run and verify local build
3. Update docs when behavior changes
4. Add or update tests when applicable

## Code Standards

- Keep functions small and cohesive
- Favor explicit names over abbreviations
- Return actionable errors
- Avoid introducing breaking changes without discussion

## Reporting Issues

Use the issue templates and include:

1. Current behavior
2. Expected behavior
3. Reproduction steps
4. Environment details

## Security

Do not post secrets, tokens, credentials, or PHI in issues or PRs.

For sensitive security reports, contact the maintainers privately.
