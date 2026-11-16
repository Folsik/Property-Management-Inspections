# Jobs

Claim work in a transaction using a query shaped like:

```sql
SELECT id FROM jobs WHERE state='pending' AND run_after <= now()
ORDER BY run_after,created_at
FOR UPDATE SKIP LOCKED LIMIT 1;
```

Mark running and assign a lease before committing. Reclaim expired leases with a separate policy. Workers are not yet implemented.
