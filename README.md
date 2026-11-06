# Mini Wallet Service

A small wallet backend: create wallets, add money from a payment gateway, send P2P
transfers, and view transaction history. It is written in Go with PostgreSQL.

## Run it

```bash
docker compose up
```

That is the only command needed. The API image is built on the first run, and the
API runs on **http://localhost:8080**. Postgres starts first, and migrations run
automatically when the API starts.

- After changing the code, rebuild with `docker compose up --build`.

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

### Examples

Real requests and responses from a fresh database.

#### Create a wallet

```bash
curl -X POST localhost:8080/api/v1/wallets \
  -d '{"user_id":"u_1001","phone":"01711000001"}'
```

`201 Created`

```json
{
  "wallet_id": "56055a57-787f-49e0-bd63-0185b9bc131d",
  "user_id": "u_1001",
  "phone": "01711000001",
  "currency": "BDT",
  "balance": 0,
  "created_at": "2026-11-06T16:01:36.220424Z"
}
```

| Case | Status | Error code |
|---|---|---|
| The user or phone already has a wallet | 409 | `DUPLICATE_WALLET` |
| Phone is not like `01711000001` | 400 | `VALIDATION_ERROR` |

#### Get a wallet

```bash
curl localhost:8080/api/v1/wallets/56055a57-787f-49e0-bd63-0185b9bc131d
```

`200 OK` (after the add-money and transfer below)

```json
{
  "wallet_id": "56055a57-787f-49e0-bd63-0185b9bc131d",
  "user_id": "u_1001",
  "phone": "01711000001",
  "currency": "BDT",
  "balance": 40000,
  "created_at": "2026-11-06T16:01:36.220424Z"
}
```

An unknown or malformed id returns `404 WALLET_NOT_FOUND`.

#### Add money

```bash
curl -X POST localhost:8080/api/v1/wallets/56055a57-787f-49e0-bd63-0185b9bc131d/add-money \
  -H 'Idempotency-Key: am-1' \
  -d '{"amount":50000,"gateway_ref":"GW-20261105-9981"}'
```

`201 Created`

```json
{
  "transaction_id": 1,
  "wallet_id": "56055a57-787f-49e0-bd63-0185b9bc131d",
  "amount": 50000,
  "gateway_ref": "GW-20261105-9981",
  "balance_after": 50000,
  "created_at": "2026-11-06T16:01:36.373699Z"
}
```

Sending the exact same request again returns the same `201` and the same body,
with the header `Idempotent-Replayed: true`. The wallet is not credited twice.

| Case | Status | Error code |
|---|---|---|
| Same `Idempotency-Key`, different body | 422 | `IDEMPOTENCY_KEY_REUSED` |
| Same `gateway_ref` under a new key (or another wallet) | 409 | `DUPLICATE_GATEWAY_REF` |
| `"amount": 100.5`, `1e3` or `"500"` | 400 | `INVALID_AMOUNT` |
| `"amount": 99` or more than `2500000` | 400 | `INVALID_AMOUNT` |
| No `Idempotency-Key` header | 400 | `MISSING_IDEMPOTENCY_KEY` |

#### Transfer money

```bash
curl -X POST localhost:8080/api/v1/transfers \
  -H 'Idempotency-Key: tr-1' \
  -d '{"from_wallet_id":"56055a57-787f-49e0-bd63-0185b9bc131d",
       "to_wallet_id":"971c93db-dd20-4571-801d-433fb86456e3",
       "amount":10000,"note":"Lunch"}'
```

`201 Created`

```json
{
  "transfer_id": "b8a4d6ee-17fb-4a6a-bdb4-3e8c7ea8e426",
  "from_wallet_id": "56055a57-787f-49e0-bd63-0185b9bc131d",
  "to_wallet_id": "971c93db-dd20-4571-801d-433fb86456e3",
  "amount": 10000,
  "note": "Lunch",
  "status": "COMPLETED",
  "created_at": "2026-11-06T16:01:36.606424Z"
}
```

| Case | Status | Error code |
|---|---|---|
| Sender's balance is too low | 422 | `INSUFFICIENT_BALANCE` |
| `from_wallet_id` equals `to_wallet_id` | 400 | `SAME_WALLET_TRANSFER` |
| Sender or receiver does not exist | 404 | `WALLET_NOT_FOUND` (message says which one) |
| Same `Idempotency-Key`, different body | 422 | `IDEMPOTENCY_KEY_REUSED` |
| No `Idempotency-Key` header | 400 | `MISSING_IDEMPOTENCY_KEY` |

#### Transaction history

```bash
curl localhost:8080/api/v1/wallets/56055a57-787f-49e0-bd63-0185b9bc131d/transactions
```

`200 OK`, newest first

```json
{
  "data": [
    {
      "transaction_id": 3,
      "type": "TRANSFER_OUT",
      "amount": 10000,
      "balance_after": 40000,
      "counterparty_wallet_id": "971c93db-dd20-4571-801d-433fb86456e3",
      "reference_id": "b8a4d6ee-17fb-4a6a-bdb4-3e8c7ea8e426",
      "created_at": "2026-11-06T16:01:36.606424Z"
    },
    {
      "transaction_id": 1,
      "type": "ADD_MONEY",
      "amount": 50000,
      "balance_after": 50000,
      "counterparty_wallet_id": null,
      "reference_id": "GW-20261105-9981",
      "created_at": "2026-11-06T16:01:36.373699Z"
    }
  ],
  "pagination": { "page": 1, "page_size": 20, "total_items": 2, "total_pages": 1 }
}
```

With filters, wallet B sees the same transfer as `TRANSFER_IN`:

```bash
curl 'localhost:8080/api/v1/wallets/971c93db-dd20-4571-801d-433fb86456e3/transactions?type=TRANSFER_IN&from=2026-11-01&to=2026-11-30&page=1&page_size=20'
```

```json
{
  "data": [
    {
      "transaction_id": 4,
      "type": "TRANSFER_IN",
      "amount": 10000,
      "balance_after": 10000,
      "counterparty_wallet_id": "56055a57-787f-49e0-bd63-0185b9bc131d",
      "reference_id": "b8a4d6ee-17fb-4a6a-bdb4-3e8c7ea8e426",
      "created_at": "2026-11-06T16:01:36.606424Z"
    }
  ],
  "pagination": { "page": 1, "page_size": 20, "total_items": 1, "total_pages": 1 }
}
```

- `ADD_MONEY` uses the `gateway_ref` as `reference_id`; transfers use the `transfer_id`.
- `transaction_id` can have gaps (here there is no 2). A rolled-back attempt, such as
  a rejected duplicate `gateway_ref`, still uses up a sequence number. Ids show order,
  not a continuous count.
- A bad filter, for example `?page_size=500`, returns `400 VALIDATION_ERROR`.

#### Health and unknown routes

```bash
curl localhost:8080/health          # 200 {"status":"ok"}
curl localhost:8080/api/v1/nope     # 404 {"error":{"code":"NOT_FOUND","message":"route not found"}}
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

## Run the tests

**Option A: everything in Docker (no Go needed)**

```bash
docker compose --profile test run --rm tests
```

**Option B: with Go 1.25 installed locally**

```bash
docker compose up -d db
TEST_DATABASE_URL="postgres://wallet:wallet@localhost:5432/wallet_test?sslmode=disable" \
  go test -race -count=1 ./...
```

All tests live in `tests/`, split by kind:

```
tests/
  unit/                 no database needed: go test ./tests/unit/...
    domain/               validation rules, error codes
    service/              business logic against an in-memory fake repository
    handler/              routing, request rejection, history query parsing
    middleware/           panic recovery, request logging
    utils/                JSON decoding, error-to-status mapping, query parsing
    config/               environment variables
  integration/          real HTTP requests against a real Postgres
```

- **Unit tests** test each layer through its public API only. The service
  tests use a fake store (`tests/unit/service/fake_store_test.go`) that
  implements the repository interfaces in memory and imitates rollback. They check
  balance rules, locking both wallets together, full rollback when any step of a
  transfer or add-money fails, and idempotency (replay, reuse, released keys).
- **Integration tests** (`tests/integration/`) cover every rule below, including the
  concurrency cases: double-spend races, the same idempotency key sent 20 times at
  once, opposite transfers A→B and B→A and a three-way cycle A→B→C→A at the same
  time (no deadlock), the same `gateway_ref` sent under different keys, and mixed
  concurrent operations that must conserve money and keep every wallet's history
  consistent with its balance. They also check the repository directly: `FOR UPDATE`
  really blocks a second transaction, and the `CHECK (balance >= 0)` constraint
  refuses a negative balance even if the service check were skipped. They use a
  separate database, `wallet_test`, and skip themselves when `TEST_DATABASE_URL` is
  not set.

## How money safety works

1. **One database transaction per money operation.** A transfer's debit, credit,
   transfer record, both ledger entries, and its idempotency record are committed
   together. If any step fails, all of them roll back.
2. **Row locks prevent double-spending.** A transfer locks both wallets with
   `SELECT ... FOR UPDATE` before it checks the balance. A second transfer from the
   same wallet waits, then sees the updated balance.
3. **No deadlocks.** A transfer locks both wallets in one query,
   `SELECT ... WHERE id = ANY(...) ORDER BY id FOR UPDATE`, so every transfer takes
   the locks in the same (wallet id) order. A→B and B→A, or a cycle such as A→B,
   B→C, C→A, running at the same time just wait for each other briefly instead of
   deadlocking.
4. **The database is the safety net.** `CHECK (balance >= 0)` and
   `CHECK (from <> to)` constraints, plus unique indexes on `user_id`, `phone`, and
   `gateway_ref`, hold even if the application code had a bug.
5. **No floats anywhere.** Amounts are `BIGINT` in the database and `int64` in Go.
   The JSON parser rejects `100.5`, `1e3`, and `"100"`.

### Idempotency

Implemented in [`internal/service/idempotency.go`](internal/service/idempotency.go):

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

Each request flows through three layers, each in its own folder:

```
HTTP request ──► routes ──► handler ──► service ──► repository ──► PostgreSQL
                            (HTTP)      (business     (SQL)
                                         logic)
```

- **handler** reads the request and writes the response. No business rules.
- **service** holds every business rule (limits, balance checks, locking, idempotency).
  It never imports HTTP or the database driver.
- **repository** holds all SQL. The service uses it only through the interfaces in
  `service/repository.go`, so another database could be added in
  `repository/<name>/` without touching the business logic.

```
cmd/api/main.go            entry point: config, DB connection, migrations, wiring, graceful shutdown
internal/
  routes/                  every URL in one place, plus the middleware chain
  handler/                 one file per resource (wallet, transfer, transaction, health)
  middleware/              panic recovery, request logging
  service/                 business logic, one file per feature
    repository.go            the repository interfaces the service needs
    idempotency.go           exactly-once execution for add money and transfers
  repository/
    postgres/              PostgreSQL implementation: connection, migrations, all SQL
  domain/                  entities, error codes, validation rules (no dependencies)
  dto/                     request bodies, service inputs, response shapes
  config/                  settings from environment variables
  utils/                   shared HTTP helpers: JSON decoding, error responses, query params
migrations/                SQL schema (embedded into the binary)
tests/
  unit/                    one folder per package, no database needed
  integration/             real HTTP requests against a real PostgreSQL
```
