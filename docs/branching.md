# Branch workflow

Use exactly two active development branches:

1. `feat/bootstrap-foundation` is the integration and validation branch. Start new feature changes here or merge work here first.
2. `main` is the stable branch. Advance it only after Go checks, PostgreSQL integration, Next.js checks, Docker Compose smoke tests, and code review succeed.

The older `feat/nextjs-web-app` branch was fast-forward integrated into
`feat/bootstrap-foundation` with no conflict. `feat/nextjs-web-platform`
was never developed beyond the original bootstrap commit.

The repository's previous PR #2 targeted `main` and is superseded by this integration approach. No force-push is needed.

## Verify locally

```sh
git checkout feat/bootstrap-foundation
git pull --ff-only
docker compose config
docker compose up --build -d
curl -fsS http://localhost:8080/healthz
curl -fsS http://localhost:3000/login
```

To start the web application use `http://localhost:3000`; Go runs on port 8080.
