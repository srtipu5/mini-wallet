# Mini Wallet Service

A small wallet backend: create wallets, add money from a payment gateway, send P2P
transfers, and view transaction history. It is written in Go with PostgreSQL.

## Run it

```bash
make up          # or: docker compose up --build
```

The API runs on **http://localhost:8080**. Postgres starts first, and migrations
run automatically when the API starts. To use other host ports, set them first:
`API_PORT=9090 DB_PORT=5433 make up`.

## Make commands

The [`Makefile`](Makefile) wraps the common Docker and Go commands. Run `make` on
its own to list them all.

| Command | What it does |
|---|---|
| `make up` | Build and start the API and Postgres in the foreground (`docker compose up --build`) |
| `make up-d` | Same as `make up`, but in the background |
| `make down` | Stop and remove the containers. The database data is kept |
| `make restart` | Run `make down`, then `make up-d` |
| `make logs` | Follow the API logs |
| `make ps` | Show container status |
| `make db` | Start only Postgres, in the background |
| `make test` | Run the full test suite in Docker. You don't need Go installed |
| `make test-local` | Start Postgres, then run `go test -race ./...` with your local Go |
| `make test-unit` | Run only the unit tests (`internal/...`). No database needed |
| `make fmt` | Format the Go code (`go fmt`) |
| `make vet` | Run `go vet` |
| `make psql` | Open a `psql` shell in the database container |
| `make clean` | Stop the containers **and delete the volumes**. This wipes the database |

## Run the tests

**Option A: everything in Docker (no Go needed)**

```bash
make test        # or: docker compose --profile test run --rm tests
```

**Option B: with Go 1.25 installed locally**

```bash
make test-local
```

This is the same as running:

```bash
docker compose up -d db
TEST_DATABASE_URL="postgres://wallet:wallet@localhost:5432/wallet_test?sslmode=disable" \
  go test -race -count=1 ./...
```

There are two kinds of tests:

- **Unit tests** (`internal/...`) check validation and parsing, with no database.
- **Integration tests** (`tests/`) send real HTTP requests to a real Postgres database.
  They cover every rule below, including the concurrency cases: double-spend races,
  the same idempotency key sent 20 times at once, opposite transfers A→B and B→A at
  the same time (no deadlock), and the same `gateway_ref` sent under different keys.
  They use a separate database, `wallet_test`, and skip themselves when
  `TEST_DATABASE_URL` is not set.

## API

All amounts are integers in **poisha** (100 poisha = 1 BDT). Each add-money or
transfer must be between **100** and **2,500,000** poisha.

| Method | Path | Notes |
|---|---|---|
| `POST` | `/api/v1/wallets` | `{"user_id","phone"}` → **201**. Returns 409 `DUPLICATE_WALLET` if the user or phone already has a wallet |
| `GET` | `/api/v1/wallets/{wallet_id}` | **200**, the wallet with its current balance |
| `POST` | `/api/v1/wallets/{wallet_id}/add-money` | Needs an `Idempotency-Key` header. Body `{"amount","gateway_ref"}` → **201** |
| `POST` | `/api/v1/transfers` | Needs an `Idempotency-Key` header. Body `{"from_wallet_id","to_wallet_id","amount","note"?}` → **201** |
| `GET` | `/api/v1/wallets/{wallet_id}/transactions` | **200**. Optional query parameters: `type`, `from`, `to`, `page`, `page_size` |
| `GET` | `/health` | Checks that the database is reachable |

Example:

```bash
curl -X POST localhost:8080/api/v1/wallets -d '{"user_id":"u_1001","phone":"01711000001"}'

curl -X POST localhost:8080/api/v1/wallets/<wallet_id>/add-money \
  -H 'Idempotency-Key: 6f1c...' -d '{"amount":50000,"gateway_ref":"GW-20261105-9981"}'

curl -X POST localhost:8080/api/v1/transfers -H 'Idempotency-Key: 9a2e...' \
  -d '{"from_wallet_id":"<a>","to_wallet_id":"<b>","amount":10000,"note":"Lunch"}'

curl 'localhost:8080/api/v1/wallets/<wallet_id>/transactions?type=TRANSFER_OUT&from=2026-11-01&to=2026-11-07&page=1&page_size=20'
```

The history response looks like this:

```json
{
  "data": [
    {"transaction_id": 2, "type": "TRANSFER_OUT", "amount": 10000, "balance_after": 40000,
     "counterparty_wallet_id": "<b>", "reference_id": "<transfer_id>", "created_at": "..."}
  ],
  "pagination": {"page": 1, "page_size": 20, "total_items": 1, "total_pages": 1}
}
```

### Errors

Every error uses the same envelope: `{"error": {"code": "...", "message": "..."}}`.

| Code | HTTP | When |
|---|---|---|
| `VALIDATION_ERROR` | 400 | Bad JSON, missing or invalid fields, unknown fields, bad query parameters |
| `INVALID_AMOUNT` | 400 | Amount is not an integer, or is outside 100–2,500,000 |
| `MISSING_IDEMPOTENCY_KEY` | 400 | The `Idempotency-Key` header is missing or empty |
| `SAME_WALLET_TRANSFER` | 400 | `from_wallet_id` equals `to_wallet_id` |
| `WALLET_NOT_FOUND` | 404 | The wallet does not exist (a malformed id is also reported as not found) |
| `DUPLICATE_WALLET` | 409 | The user or phone already has a wallet |
| `DUPLICATE_GATEWAY_REF` | 409 | This gateway payment has already been credited |
| `INSUFFICIENT_BALANCE` | 422 | The sender's balance is lower than the amount |
| `IDEMPOTENCY_KEY_REUSED` | 422 | The same key was sent with a different request body |
| `NOT_FOUND` | 404 | Unknown route |
| `INTERNAL_ERROR` | 500 | Unexpected failure. Details are logged, never returned to the client |

## How money safety works

1. **One database transaction per money operation.** A transfer's debit, credit,
   transfer record, both ledger entries, and its idempotency record are committed
   together. If any step fails, all of them roll back.
2. **Row locks prevent double-spending.** A transfer locks both wallets with
   `SELECT ... FOR UPDATE` before it checks the balance. A second transfer from the
   same wallet waits, then sees the updated balance.
3. **No deadlocks.** Both wallets are always locked in the same (sorted) order. So
   A→B and B→A running at the same time just wait for each other briefly instead of
   deadlocking.
4. **The database is the safety net.** `CHECK (balance >= 0)` and
   `CHECK (from <> to)` constraints, plus unique indexes on `user_id`, `phone`, and
   `gateway_ref`, hold even if the application code had a bug.
5. **No floats anywhere.** Amounts are `BIGINT` in the database and `int64` in Go.
   The JSON parser rejects `100.5`, `1e3`, and `"100"`.

### Idempotency

Implemented in [`internal/application/idempotency.go`](internal/application/idempotency.go):

1. Inside the money transaction, the service runs
   `INSERT INTO idempotency_keys ... ON CONFLICT DO NOTHING`.
2. **New key:** the service does the work, saves the response in the same row, and commits.
3. **Key already used with the same body:** the service returns the saved response
   (same status, same body) and adds the header `Idempotent-Replayed: true`.
4. **Key already used with a different body:** 422 `IDEMPOTENCY_KEY_REUSED`. The body
   is compared by a SHA-256 hash of the parsed request, so whitespace and field order
   do not matter.
5. **Same key sent at the same moment:** the second `INSERT` waits on the primary key
   until the first transaction finishes. It then sees the committed row and replays it.
   The work runs exactly once.

## Assumptions

- **Only successful results are stored for idempotency.** If a request fails (for
  example `INSUFFICIENT_BALANCE`), nothing changed and the key is released, so the
  client can retry with the same key after fixing the cause. Validation errors are
  returned before the key is checked.
- **Keys are scoped per operation.** Add-money and transfers each have their own key
  space. The add-money hash includes the wallet id, so reusing a key for another
  wallet returns 422.
- **Phone numbers** must be in the Bangladeshi local format, `01[3-9]XXXXXXXX`. A
  phone number can belong to only one wallet.
- **`gateway_ref` is unique across the whole system**, not per wallet. One gateway
  payment can never be credited twice, not even to a different wallet.
- **Add-money returns 201**, because it creates a transaction record.
- **Date filters:** `from` and `to` accept an ISO-8601 date (`2026-11-05`) or a
  date-time (`2026-11-05T10:00:00Z`). The range is half-open, `[from, to)`. A
  date-only `to` includes the whole of that day. Date-only values use Bangladesh
  time (UTC+6).
- **Pagination:** `page_size` defaults to 20. Values above 100 return a 400 instead
  of being silently capped.
- **History order:** newest first, ordered by the ledger id. The id is assigned while
  the wallet is locked, so the order always matches the order of balance changes.
- **Idempotency keys never expire.** In production you would add a TTL clean-up job.
- **No authentication.** It was not part of the assignment.

## Project layout

```
cmd/api/                       entry point: config, DB connection, migrations, HTTP server, graceful shutdown
internal/domain/               models, error codes, validation rules (no dependencies)
internal/application/          business logic: transfers, add money, idempotency, history
internal/repository/           all SQL (pgx)
internal/handler/              HTTP: routing, JSON parsing, error → status mapping
internal/middleware/           panic recovery, request logging
internal/infrastructure/postgres/  connection pool and migration runner
migrations/                    SQL schema (embedded into the binary)
tests/                         end-to-end tests against real Postgres
```
