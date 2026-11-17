# Media

Media objects require tenant authorization, size/type limits and server-side verification after direct S3/R2 upload. Never expose bucket credentials to mobile/web clients.

Implementation: [docs/media-lifecycle.md](../../docs/media-lifecycle.md). Go API routes live in internal/api/media*.go and the background cleanup runner is started from internal/app/app.go.
