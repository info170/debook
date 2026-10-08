# Безопасность

- Private key Autheo хранится только в локальном `.env` и передаётся workflow; его нельзя коммитить или писать в логи.
- API проверяет, что `wallet_address` является корректным EVM-адресом, до создания брони.
- Для API включены таймауты, лимит тела POST-запроса 64 KiB, базовый rate limit 120 запросов в минуту на IP и security-заголовки `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`.
- CORS разрешает только локальные Swagger origins `localhost:8081` и `127.0.0.1:8081`.
- В blockchain proof записывается hash идентификатора брони без PII; персональные данные не передаются в Autheo.
- Для production необходимы Docker secrets/secret manager, HTTPS, внешний rate limiter, полноценная аутентификация и ротация ключей.
- `RATE_LIMIT_PER_MINUTE` и `CORS_ALLOWED_ORIGINS` задаются через `.env`; для production список origins должен быть явным, без wildcard.
