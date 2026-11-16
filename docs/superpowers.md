# Superpowers setup and workflow

Upstream: https://github.com/obra/superpowers

This repository follows brainstorming → design/plan → TDD implementation → verification → code review. The upstream skill framework must be installed in the *coding agent*, not treated as an ordinary Go runtime dependency.

Codex App: Plugins → Coding → Superpowers → Install.
Codex CLI: /plugins → Superpowers → Install Plugin.
For other harnesses use upstream README instructions.

Installation could not be performed inside the initial chat execution environment because outbound Git network access was unavailable. No claim is made that a plugin was activated.

Task checkpoints:
1. Port only reusable auth, JWT, storage, lifecycle and Docker/CI.
2. Implement tenant schema and membership authorization.
3. Add OTP/PIN and org/property/inspection routes with tests.
4. Add presigned S3/R2 media uploads.
5. Run go test, go vet, go build, PostgreSQL integration tests and security review.
