# E-Wallet API (Go + Gin)

REST API for a simple e-wallet: register/login with JWT, top up, pay, transfer, and view transaction history. Built for the MNC Bank technical test, starting from [Massad/gin-boilerplate](https://github.com/Massad/gin-boilerplate) (MIT).

**Tech:** Go · Gin · PostgreSQL (gorp) · Redis · JWT (access + refresh) · Docker Compose

## Features
- **Auth:** register, login, logout, refresh token. JWTs with access/refresh secrets; token metadata stored in Redis so logout invalidates it.
- **PIN brute-force protection:** after 5 unsuccessful logins for one phone number, or 20 from one client IP, within 15 minutes, that phone or IP gets `429 Too Many Requests` (with `Retry-After`) for 15 minutes. A successful login clears the phone's count.
- **Wallet operations** (JWT-protected): `POST /v1/payment`, `POST /v1/transfer`.
- **Top-ups:** only from the payment provider's signed webhook or an admin, within per-user limits and with an audit trail (see [Top-ups](#top-ups)). Users cannot top up their own wallet.
- **Ledger:** every operation writes a `transactions` row with type (`CREDIT`/`DEBIT`), amount, `balance_before`, `balance_after`, status and remarks.
- **History:** `GET /v1/transactions` returns the user's transactions.
- Middleware: CORS, request ID, gzip; custom form validators.
- PostgreSQL schema with `created_at` / `updated_at` triggers (`db/database.sql`) and example data (`db/example_data.sql`).

## Run locally
```bash
cp .env_copy .env            # adjust DB / Redis, replace every CHANGE_ME secret (see below)
docker compose up -d         # PostgreSQL + Redis
psql -U postgres -h localhost < db/database.sql
psql -U postgres -h localhost golang_gin_db < db/example_data.sql   # optional
go run .
```
Set `SSL=FALSE` in `.env` unless you generated certs with `generate-certificate.sh`.

A database created from an older `db/database.sql` needs `db/migrations/001_topup_audit_and_roles.sql` (adds `user.role` and the `topup_audit` table).

Tests need no running services (they use sqlmock and an in-memory Redis): `go test ./...`.

## Secrets
`.env_copy` is public and holds placeholders only. The server refuses to start if `ACCESS_SECRET` or `REFRESH_SECRET` is missing, still starts with `CHANGE_ME`, is shorter than 32 characters, or both are the same value.

- Generate each secret separately for every environment from at least 32 random bytes, e.g. `openssl rand -base64 48`. Do not share a value between environments or between `ACCESS_SECRET` and `REFRESH_SECRET`.
- Keep real values in `.env` (git-ignored) or your deployment's secret store, never in the repository.
- `PAYMENT_WEBHOOK_SECRET` follows the same rules when set; while it is empty the top-up webhook rejects every delivery.
- `TRUSTED_PROXIES` lists the reverse proxies (IPs or CIDRs) whose `X-Forwarded-For` header is believed. Behind a load balancer, set it to the balancer's range; otherwise every client shares the balancer's IP for login rate limiting.
- Earlier versions of `.env_copy` committed concrete values for `ACCESS_SECRET`, `REFRESH_SECRET` and `REDIS_SECRET`. They stay in git history, so treat them as public: rotate any secret that was copied from this file. Rotating the JWT secrets invalidates all issued tokens, so users must log in again.

## API
| Method | Path | Auth |
|---|---|---|
| POST | `/v1/register` | – |
| POST | `/v1/login` | – |
| GET | `/v1/user/logout` | Bearer |
| POST | `/v1/token/refresh` | refresh token |
| PUT | `/v1/profile` | Bearer |
| POST | `/v1/payment` | Bearer |
| POST | `/v1/transfer` | Bearer |
| GET | `/v1/transactions` | Bearer |
| POST | `/v1/webhooks/topup` | payment-provider HMAC signature |
| POST | `/v1/admin/topup` | Bearer, `ADMIN` role |

## Top-ups
A wallet is credited only through these two routes:

- **`POST /v1/webhooks/topup`** is called by the payment provider once a top-up payment settles. Body: `{"reference": "<provider payment id>", "user_id": "...", "amount": 50000, "status": "PAID"}`; events with any other status are acknowledged and ignored. Send `X-Webhook-Timestamp: <unix seconds>` and `X-Webhook-Signature: sha256=<hex>`, where the hex is the HMAC-SHA256 of `<timestamp>.<raw body>` keyed with `PAYMENT_WEBHOOK_SECRET`. A bad signature or a timestamp more than 5 minutes off gets `401`. A `reference` that was already credited gets `200` again without a second credit, so provider retries are safe.
- **`POST /v1/admin/topup`** is for users whose `role` is `ADMIN` (grant it in SQL: `UPDATE "user" SET role = 'ADMIN' WHERE id = '...'`). Body: `{"user_id", "amount", "reference", "remarks"}`. Admins cannot credit their own wallet.

Per-user limits apply to both: `TOPUP_MAX_PER_TRANSACTION` for one credit (default 10,000,000) and `TOPUP_MAX_PER_DAY` across a rolling 24 hours (default 20,000,000). Over-limit credits get `422`. Enforce the same limits when the payment is created with the provider: a webhook credit rejected here has already been paid and must be refunded.

Every attempt, applied or rejected, is written to `topup_audit` with source, reference, actor, amount, client IP, outcome and reason. A trigger rejects `UPDATE` and `DELETE` on that table. An applied credit's ledger row points at its audit row through `transactions.top_up_id`.

## Project layout
```
router/       route table and middleware
controllers/  HTTP handlers (auth, user, transaction, top-up)
forms/        request DTOs + validators
models/       DB access and business logic
db/           connection setup, schema, migrations, example data
tests/        HTTP tests against the real router
```

## Roadmap
- Run each balance change in a single DB transaction with row locking.
- GitHub Actions CI running `go test ./...`.
