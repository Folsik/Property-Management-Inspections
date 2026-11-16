# Superpowers status

Upstream: https://github.com/obra/superpowers

## ChatGPT coding session status (16 November 2026)

- Plugin `superpowers`: **installed: true**, **status: ENABLED**, scope `GLOBAL`.
- This session can list and read live installed skills such as `using-superpowers`, `verification-before-completion`, and `finishing-a-development-branch`. **Skills are available in this session.**
- The repository does **not** vendor or submodule the framework: Superpowers is an **agent plugin**, not a Go or Next.js runtime dependency.
- Another coding harness (Codex CLI, Claude Code, etc.) may need its own installation; this ChatGPT status does not automatically enable other agents.

## Development methodology

Use brainstorming → plan → test-first implementation → verification → code review.
For branch integration, read `finishing-a-development-branch` and `verification-before-completion`. See `docs/branching.md` for the project-specific two-branch workflow.

The application's production runtime does not require Superpowers.
