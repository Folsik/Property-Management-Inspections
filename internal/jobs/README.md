# Durable background work

The implementation now lives in `runner.go`, `pipeline.go`, migrations 011–012, and the separate `cmd/worker` process. Claiming uses `SKIP LOCKED`, per-attempt UUID lease fencing, heartbeat and bounded retry/backoff. Expired final-attempt claims are moved to dead-letter; owners/managers can requeue failures through the API. Stage chaining occurs in the success acknowledgement transaction.

For deployment settings, STT adapter contract, privacy requirements, endpoint details, recovery and known limitations, see [docs/workers-stage4.md](../../docs/workers-stage4.md). No secrets or signed URLs are stored in job payloads.
