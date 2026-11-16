# OTP delivery

Real production SMS delivery uses Twilio's REST Messages API configured by:

- `TWILIO_ACCOUNT_SID` (AC + 32 hex characters)
- `TWILIO_AUTH_TOKEN`
- `TWILIO_FROM_NUMBER` (E.164)

When configured, OTP requests send actual SMS and never reveal codes in responses.
When the provider is unavailable, the server fails closed. In development only,
if no Twilio credentials are set, Go returns a local-only dev code for API testing;
the Next.js BFF intentionally strips that secret from browser responses.

Before production deployment: implement per-IP/device anti-abuse policy, SMS
spend monitoring and abuse alerts, and full secret rotation procedures.
