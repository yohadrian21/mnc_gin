# E-Wallet API (Go + Gin)

REST API for a simple e-wallet: register/login with JWT, top up, pay, transfer, and view transaction history. Built for the MNC Bank technical test, starting from [Massad/gin-boilerplate](https://github.com/Massad/gin-boilerplate) (MIT).

**Tech:** Go · Gin · PostgreSQL (gorp) · Redis · JWT (access + refresh) · Docker Compose

## Features
- **Auth:** register, login, logout, refresh token. JWTs with access/refresh secrets; token metadata stored in Redis so logout invalidates it.
- **PIN brute-force protection:** after 5 unsuccessful logins for one phone number, or 20 from one client IP, within 15 minutes, that phone or IP gets `429 Too Many Requests` (with `Retry-After`) for 15 minutes. A successful login clears the phone's count.
- **Wallet operations** (JWT-protected): `POST /v1/topup`, `POST /v1/payment`, `POST /v1/transfer`.
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

## Secrets
`.env_copy` is public and holds placeholders only. The server refuses to start if `ACCESS_SECRET` or `REFRESH_SECRET` is missing, still starts with `CHANGE_ME`, is shorter than 32 characters, or both are the same value.

- Generate each secret separately for every environment from at least 32 random bytes, e.g. `openssl rand -base64 48`. Do not share a value between environments or between `ACCESS_SECRET` and `REFRESH_SECRET`.
- Keep real values in `.env` (git-ignored) or your deployment's secret store, never in the repository.
- `TRUSTED_PROXIES` lists the reverse proxies (IPs or CIDRs) whose `X-Forwarded-For` header is believed. Behind a load balancer, set it to the balancer's range; otherwise every client shares the balancer's IP for login rate limiting.
- Earlier versions of `.env_copy` committed concrete values for `ACCESS_SECRET`, `REFRESH_SECRET` and `REDIS_SECRET`. They stay in git history, so treat them as public: rotate any secret that was copied from this file. Rotating the JWT secrets invalidates all issued tokens, so users must log in again.

## API
| Method | Path | Auth |
|---|---|---|
| POST | `/v1/register` | – |
| POST | `/v1/login` | – |
| GET | `/v1/user/logout` | Bearer |
| POST | `/v1/token/refresh` | refresh token |
| POST | `/v1/topup` | Bearer |
| POST | `/v1/payment` | Bearer |
| POST | `/v1/transfer` | Bearer |
| GET | `/v1/transactions` | Bearer |

## Project layout
```
controllers/  HTTP handlers (auth, user, transaction)
forms/        request DTOs + validators
models/       DB access and business logic
db/           connection setup, schema, example data
```

## Roadmap
- Run each balance change in a single DB transaction with row locking.
- Wallet-flow tests and GitHub Actions CI.
