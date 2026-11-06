# Project rules

Rules for anyone changing this codebase, human or AI. Add your own rules in
[My rules](#my-rules) at the bottom.

## Project

Mini Wallet: a Go 1.25 + PostgreSQL wallet backend (create wallets, add money,
P2P transfers, history). Money correctness is the top priority, ahead of
features and cleverness.

## Commands

| Task | Command |
|---|---|
| Run the API + Postgres | `make up` (foreground) or `make up-d` (background) |
| Stop | `make down` (keeps data) · `make clean` (**wipes the database**) |
| All tests (Docker) | `make test` |
| Unit tests only (no DB) | `make test-unit` |
| All tests with local Go | `make test-local` |
| Format / vet | `make fmt` · `make vet` |
| DB shell | `make psql` |

Before calling any change done: `gofmt -l .` prints nothing, `go vet ./...`
passes, and `make test` passes.

## Architecture

Keep it simple: **routes → handler → service → repository**. Each layer has one
job and its own folder. Easy to understand comes first; add abstraction only
when a real need appears.

```
routes ──► handler ──► service ──► repository/postgres
                          │
                          └── uses only the interfaces in service/repository.go
```

### Who may import whom (never break this)

| Package | May import |
|---|---|
| `domain` | standard library only |
| `dto` | `domain` |
| `service` | `domain`, `dto` (never `net/http`, never `pgx`) |
| `handler` | `service`, `dto`, `domain`, `utils` |
| `routes` | `handler`, `middleware`, `service` |
| `middleware` | `utils` |
| `repository/postgres` | `service` (interfaces), `domain`, `migrations`, `pgx`, `golang-migrate` |
| `config` | standard library only |
| `utils` | `domain` |

Check it with:
`go list -f '{{.ImportPath}} -> {{join .Imports " "}}' ./internal/...`

### Where things go

| What | Where |
|---|---|
| New route | `internal/routes/routes.go`, and nowhere else |
| HTTP handler | `internal/handler/<resource>_handler.go` (exported method on `Handler`) |
| Request body / service input / response shape | `internal/dto/<resource>.go` (`XxxRequest`, `XxxInput`, `XxxResult`) |
| Business rule | `internal/service/<feature>_service.go` |
| New repository method | Add it to the interface in `internal/service/repository.go` first, then implement it in `internal/repository/postgres/<table>_repository.go` |
| SQL | Only in `internal/repository/postgres` |
| Another database | A new sibling folder `internal/repository/<name>/` implementing the same interfaces. Only `cmd/api/main.go` chooses which one to use; `service` never changes |
| Entity, error code, validation rule | `internal/domain` |
| Shared HTTP helper | `internal/utils` (no business rules) |
| Setting from env | `internal/config` |
| Schema change | New numbered pair in `migrations/` (`00000N_name.up.sql` + `.down.sql`); never edit an applied migration |

### Layer responsibilities

- **Handler**: parse the request (`utils.DecodeJSON`, `utils.ParseAmount`,
  `utils.IdempotencyKey`), call the service, write the response with
  `utils.SendData` / `utils.SendError` / `utils.SendIdempotent`. No business
  rules, no SQL.
- **Service**: validation, business rules, transactions (`store.WithinTx`),
  idempotency (`runIdempotent`). Talks to the database only through the
  interfaces in `service/repository.go`.
- **Repository**: SQL only. Translates Postgres errors (unique and check
  violations) into `domain` errors. No business decisions.

### Keep it simple

- Prefer plain names over pattern jargon (no "ports", "adapters", "use cases"
  folders).
- No new folder, interface or layer until there is a second real use for it.
- Folders at most two levels deep under `internal/`.

## Money rules (non-negotiable)

- Amounts are `int64` poisha (100 poisha = 1 BDT). **Never use floats** for money,
  in Go, SQL or JSON parsing.
- Every add-money or transfer amount is between `domain.MinAmount` (100) and
  `domain.MaxAmount` (2,500,000).
- A balance can never go negative. Keep the `CHECK (balance >= 0)` constraint as
  the safety net, even though the service checks first.
- Every money movement (balance changes, transfer record, ledger entries and
  idempotency key) happens in **one** `store.WithinTx` call. Never split it.
- Lock wallets with `Wallets.Lock` (`SELECT ... FOR UPDATE`) **in sorted id order**
  before checking a balance. This prevents both double-spending and deadlocks.
- Every balance change writes a ledger row (`transactions`) with `balance_after`.

## Idempotency rules

- `POST .../add-money` and `POST /transfers` require an `Idempotency-Key` header
  and must go through `runIdempotent`.
- **Do not change the JSON tags** of `dto.AddMoneyInput` / `dto.TransferInput`.
  They are hashed to detect a reused key, so changing them breaks every stored key.
- Normalise input (lowercase ids, blank note → nil) **before** hashing.
- Only successful results are stored. A failed request releases its key.

## API and errors

- Every error uses the envelope `{"error": {"code", "message"}}`.
- Return `domain` errors (`domain.NewError`, `domain.Validation`, or the predefined
  `domain.ErrXxx`) from the service and repository layers. Never write HTTP status
  codes there.
- A new error code needs a constant in `internal/domain/errors.go` **and** a status
  in `statusByCode` in `internal/utils/response.go`.
- Unknown errors become a generic 500. Never leak internal error text to clients.
- JSON field names are part of the public API: `snake_case`, never renamed
  without a new API version.

## Testing rules

- All tests live in `tests/`:
  - `tests/unit/<layer>/`: no database. Use external test packages
    (`package service_test`) and test through the **public API only**.
  - `tests/integration/`: real HTTP router + real Postgres.
- **Never export a function just to test it.** Test it through the public behaviour
  instead.
- Service logic is unit-tested against the in-memory fake in
  `tests/unit/service/fake_store_test.go`. When you add a repository method, add it to the
  fake too.
- Every new rule or bug fix gets a test. Concurrency-sensitive changes (locking,
  idempotency, balances) need an integration test that runs requests concurrently
  (`runConcurrently`).

## Code style

- `gofmt`, standard Go naming, small focused functions.
- Comments explain **why**, not what.
- Use `log/slog` for logs. Never log secrets or full request bodies.
- No new dependencies without a clear reason.

## Git

- Small commits with clear messages that say what changed and why.
- Don't commit `.env` or any secret.

## My rules

<!-- Add your own rules below. They take priority over the defaults above. -->

-
