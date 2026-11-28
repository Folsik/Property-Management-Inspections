# Trusted client IPs for authentication rate limiting

The Go API limits both network source and normalized phone/email identifiers using PostgreSQL counters. Native clients connect directly and use their TCP peer IP. The BFF forwards authenticated ingress metadata only. Generic X-Forwarded-For and X-Real-IP are ignored. No new API endpoints or JSON fields are required.

## Production deployment contract

Generate **two independent** 256-bit secrets (run twice):

```sh
openssl rand -hex 32
```

- Set TRUSTED_BFF_SECRET on Go and Next.js (same value).
- Set TRUSTED_INGRESS_SECRET on Next.js and your trusted TLS ingress (different value).
- Keep these server-only secrets out of NEXT_PUBLIC_* variables, logs, version control and browser responses.
- Keep Next.js and Go privately reachable from their intended callers; protect inter-service traffic with TLS or an isolated trusted network. Direct native API ingress must itself rate-limit or provide an independently secured deployment path. Never expose a raw untrusted proxy hop.
- At the first internet-facing ingress, discard any inbound Inspectly authentication metadata. Overwrite the two ingress headers with its observed peer address and its configured secret. Do not copy client-provided forwarding chains. If a CDN/load balancer precedes ingress, configure the ingress real-IP module with that provider's exact trusted CIDRs before using its observed address.

Example Nginx location (inside an existing HTTPS server; inject actual secret through secure deployment templating, not source control):

```nginx
location / {
    proxy_pass http://web:3000;
    proxy_set_header Host $host;
    proxy_set_header X-Inspectly-Ingress-IP $remote_addr;
    proxy_set_header X-Inspectly-Ingress-Secret "<TRUSTED_INGRESS_SECRET>";
    proxy_set_header X-Inspectly-Client-IP "";
    proxy_set_header X-Inspectly-Proxy-Secret "";
    proxy_set_header X-Forwarded-For "";
    proxy_set_header X-Real-IP "";
}
```

Next.js verifies the ingress proof before sending a separate proxy proof and a single IP to Go. Go verifies its own proof in constant time and validates/canonicalizes the IP (IPv6 /64). Duplicate or malformed headers, missing secrets or mismatched proofs never grant forwarded-IP trust. Without authenticated metadata, the peer IP is used; unsecured direct BFF access therefore shares the proxy's bucket. This is an intentional safe fallback, not a production substitute for ingress configuration. The existing local Compose workflow remains unchanged when secrets are absent.

## Limits

| Endpoint | Identifier scope | Fixed window |
|---|---:|---:|
| OTP request | 3 | 15 min |
| OTP verify | 20 | 10 min |
| Password login | 20 | 10 min |
| Legacy PIN login | 20 | 10 min |
| Password recovery request | 5 | 30 min |
| Email OTP request | 5 | 15 min |
| Email OTP verify | 20 | 10 min |

Existing per-IP limits and account-level controls remain in place. Identifier checks happen after canonical input validation and before account lookup, and do not depend on existence. Phone and email aliases of one account retain distinct identifier buckets; the existing account lockout remains the cross-alias control until priority 2. PostgreSQL stores hashed counter scopes; hashing is not anonymization of low-entropy phones/email addresses. Counter retention is priority 2. No shared secret or identifier is returned in error responses.

Verify at deployment that authenticated calls from two separate external IPs produce distinct IP buckets, and that an unauthenticated caller cannot alter the bucket with either standard forwarding headers or custom Inspectly headers. Do not print secrets while testing.
