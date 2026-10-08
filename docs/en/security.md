# Security

- The Autheo private key is kept only in the local `.env` and passed to workflow; it must never be committed or logged.
- The API validates that `wallet_address` is a valid EVM address before creating a booking.
- The API has read/write timeouts, a 64 KiB POST body limit, a basic per-IP rate limit of 120 requests per minute, and security headers: `X-Content-Type-Options`, `X-Frame-Options`, and `Referrer-Policy`.
- CORS allows only the local Swagger origins `localhost:8081` and `127.0.0.1:8081`.
- Blockchain proofs contain a hash of the booking identity without PII; personal data is not sent to Autheo.
- Production still requires Docker secrets/secret manager, HTTPS, an external rate limiter, full authentication, and key rotation.
- `RATE_LIMIT_PER_MINUTE` and `CORS_ALLOWED_ORIGINS` are configurable through `.env`; production should use an explicit origin list without wildcards.
