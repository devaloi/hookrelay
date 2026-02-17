# Agent Prompt — hookrelay

You are building a portfolio project. Your job is to produce clean, professional, senior-engineer-quality code that is ready to post publicly on GitHub.

---

## Docs to Read First

Before writing a single line of code, read all three docs in this folder:

1. `docs/G07-go-webhook-relay.md` — The project spec. Architecture, phases, data model, API design, commit plan.
2. `docs/github-portfolio.md` — Quality bar and Definition of Done. This sets the standard.
3. `docs/github-portfolio-checklist.md` — Pre-posting checklist. Every box must be checked before you're done.

---

## Rules

### Commit discipline
- One commit per logical unit of work. Follow the commit plan in the spec.
- Conventional commit messages: `feat:`, `fix:`, `test:`, `refactor:`, `docs:`, `chore:`.
- No WIP commits. No "update" commits. No empty commits.
- Write real commit messages: `feat: add SQLite queue with migrations` not `feat: add stuff`.

### Code quality
- Write the code a senior engineer with 25 years of experience would write.
- Interfaces where they add testability. No god functions. Clean separation of concerns.
- Error handling everywhere — wrap errors with context (`fmt.Errorf("doing X: %w", err)`).
- Tests are real: table-driven, test behavior not implementation, cover happy path AND error cases.
- Lint clean. `golangci-lint run` must pass with zero issues.

### What NOT to do
- Don't skip phases. Don't combine phases. Work through them in order.
- Don't leave TODO/FIXME/HACK comments in the code.
- Don't commit secrets, personal data, or hardcoded paths.
- Don't write fake tests that just assert `true`. Tests must test real behavior.
- Don't skip the refactoring phase — it's not optional polish, it's core quality.
- Don't commit `.DS_Store` or any generated files.
- Don't use Docker. No Dockerfile, no docker-compose. Just `go build` and `go run`.

---

## GitHub Username

The GitHub username is **devaloi**. For Go module paths, use `github.com/devaloi/hookrelay`. All internal imports must use this module path. Do not guess or use any other username.

## Start

Read the three docs. Then begin Phase 1 from `docs/G07-go-webhook-relay.md`.
