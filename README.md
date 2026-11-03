# E-Wallet API (Go + Gin)

REST API for a simple e-wallet: register/login with JWT, top up, pay, transfer, and view transaction history. Built for the MNC Bank technical test, starting from [Massad/gin-boilerplate](https://github.com/Massad/gin-boilerplate) (MIT).

**Tech:** Go · Gin · PostgreSQL (gorp) · Redis · JWT (access + refresh) · Docker Compose

## Features
- **Auth:** register, login, logout, refresh token. JWTs with access/refresh secrets; token metadata stored in Redis so logout invalidates it.
- **Wallet operations** (JWT-protected): `POST /v1/topup`, `POST /v1/payment`, `POST /v1/transfer`.
- **Ledger:** every operation writes a `transactions` row with type (`CREDIT`/`DEBIT`), amount, `balance_before`, `balance_after`, status and remarks.
- **History:** `GET /v1/transactions` returns the user's transactions.
- Middleware: CORS, request ID, gzip; custom form validators.
- PostgreSQL schema with `created_at` / `updated_at` triggers (`db/database.sql`) and example data (`db/example_data.sql`).

## Run locally
```bash
cp .env_copy .env            # adjust DB / Redis / JWT secrets
docker compose up -d         # PostgreSQL + Redis
psql -U postgres -h localhost < db/database.sql
psql -U postgres -h localhost golang_gin_db < db/example_data.sql   # optional
go run .
```
Set `SSL=FALSE` in `.env` unless you generated certs with `generate-certificate.sh`.

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
