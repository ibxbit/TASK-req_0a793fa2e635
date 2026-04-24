# Helios — REST API specification

This document is the authoritative reference for Helios' HTTP API as
implemented in `main.go` + `internal/handlers/*` + `internal/auth`. Every
endpoint listed here is wired to a Gin handler in this repository; any
route not in this file is not served.

> **If you are writing a client, please keep in mind**
> — base URL is `/api/v1`,
> — authentication is a **session cookie** (`helios_session`), not a bearer
>   token,
> — write endpoints require an `Idempotency-Key` header for safe retries,
> — all JSON bodies are UTF-8 with Content-Type `application/json`.

---

## 1. Transport and base URL

| Property           | Value                                |
| ------------------ | ------------------------------------ |
| Base URL           | `http://localhost:8080/api/v1`       |
| API prefix         | `/api/v1`                            |
| Content-Type (in)  | `application/json; charset=utf-8`    |
| Content-Type (out) | `application/json; charset=utf-8`    |
| CORS AllowOrigins  | `http://localhost:5173`, `http://localhost` |
| CORS credentials   | allowed — cookies are required       |

Frontend traffic goes through the nginx reverse proxy defined in
`frontend/nginx.conf`, which forwards `/api/*` to `http://backend:8080/api/*`
over the compose network. The prefix is stable — clients can safely
hard-code `/api/v1` as their base.

---

## 2. Authentication model

### 2.1 Session cookie

Helios uses a **session cookie** named `helios_session`. Clients:

1. `POST /auth/login` with JSON `{ "username", "password" }`. A valid
   login sets `Set-Cookie: helios_session=<random-id>; Path=/; HttpOnly;
   SameSite=Lax; Max-Age=1800`.
2. Include that cookie on every subsequent request. Browsers do this
   automatically when `fetch(..., { credentials: 'include' })` is used.
3. `POST /auth/logout` destroys the server-side session and clears the cookie.

There is **no bearer token**. The `Authorization:` header is ignored.

The cookie is configured as:

| Flag           | Value                               |
| -------------- | ----------------------------------- |
| `HttpOnly`     | yes (JS cannot read it)             |
| `SameSite`     | `Lax`                               |
| `Secure`       | `false` by default; set `HELIOS_COOKIE_SECURE=1` when serving over HTTPS |
| `Max-Age`      | 1800 seconds (30 minutes, matches idle timeout) |

### 2.2 Idle timeout

The session store lives in `internal/auth/session.go`:

- `IdleTimeout = 30 * time.Minute`
- `LastActiveAt` is touched on every authenticated request.
- Sessions expire 30 minutes after the last request.
- A background sweeper runs every 5 minutes to evict stale entries.

A request bearing an expired cookie receives `401 Unauthorized` and the
cookie is destroyed server-side.

### 2.3 RBAC roles

Six roles are seeded by `internal/auth/bootstrap.go`. Demo credentials:

| Role                | Username   | Password      | Scope summary                                                                         |
| ------------------- | ---------- | ------------- | ------------------------------------------------------------------------------------- |
| `administrator`     | `admin`    | `admin123`    | Full access.                                                                          |
| `content_editor`    | `editor`   | `editor123`   | CRUD on dynasties/authors/poems/excerpts/tags.                                        |
| `reviewer`          | `reviewer` | `reviewer123` | Moderate reviews; list/assign/resolve complaints.                                     |
| `marketing_manager` | `marketer` | `marketer123` | CRUD on campaigns/coupons/pricing-rules/member-tiers; back-office quotes.             |
| `crawler_operator`  | `crawler`  | `crawler123`  | Create and control crawl jobs.                                                        |
| `member`            | `member`   | `member123`   | Regular end-user: public search, own reviews/complaints, content-pack download.       |

Enforcement: `auth.AuthRequired()` gates "must be authenticated", and
`auth.RequireRole(roles...)` narrows down to specific role names. Each
endpoint below lists its exact RBAC gate.

### 2.4 Idempotency

Every mutating endpoint (`POST`, `PUT`, `PATCH`, `DELETE`) accepts an
optional `Idempotency-Key` header. The server remembers `(user_id, method,
path, key)` for 24 hours and replays the cached 2xx response for
duplicate requests — protecting against network retries. Add a random
UUID per write:

```http
Idempotency-Key: 3a7b-…
```

Replays return the cached body with an extra `Idempotent-Replay: true`
header.

---

## 3. Error envelope

All non-2xx responses carry a JSON body with an `error` field:

```json
{ "error": "human readable message" }
```

Shape conventions:

| Status | When                                                                  |
| ------ | --------------------------------------------------------------------- |
| 400    | Malformed JSON, missing required fields, validation rule violation.   |
| 401    | No session cookie, session expired, or invalid credentials.           |
| 403    | Authenticated, but the caller's role is not permitted for the action. |
| 404    | Entity not found (distinct from 400 for bad id format).               |
| 409    | Conflict (duplicate unique key, pending approval, state mismatch).    |
| 410    | Gone — revision beyond the 30-day retention window.                   |
| 429    | Rate limited (currently used only by login brute-force guard).        |
| 500    | Internal server error. Raw driver error is logged, never returned.    |

---

## 4. Endpoint reference

Every section shows: **method + path**, **auth requirement**, **request
body / query params**, and a representative **success response**. All
paths are rooted at `/api/v1`.

### 4.1 Health

```
GET /health
```

Public (no auth). Response 200:
```json
{ "status": "ok", "db": "up" }
```
Response 503 (degraded DB): `{ "status": "degraded", "db": "down" }`.

### 4.2 Auth — `/auth`

| Method | Path            | Auth | Body                                  | Success                 |
| ------ | --------------- | ---- | ------------------------------------- | ----------------------- |
| POST   | `/auth/login`   | —    | `{ username, password }`              | 200 + Set-Cookie + user |
| POST   | `/auth/logout`  | —    | _empty_                               | 200 + clears cookie     |
| GET    | `/auth/me`      | any  | —                                     | 200 + user              |
| POST   | `/auth/register`| —    | `{ username, password, email? }`      | 201 + user (role=member)|

Login response:
```json
{ "user": { "id": 1, "username": "admin", "role": "administrator" } }
```

Login failure cases: 400 bad body, 401 invalid credentials, 403 account
not active, 429 too many failed attempts.

`/auth/register` always creates a `member`-role account — staff roles are
seeded, not self-registered.

### 4.3 Content — core entities

Four resources follow the same CRUD shape: **dynasties**, **authors**,
**poems**, **excerpts**, **tags** (aliased to `genres` with `kind=tag`).

Read endpoints: any authenticated user. Write endpoints: `administrator`
+ `content_editor`.

For every resource `R ∈ {dynasties, authors, poems, excerpts, tags}`:

| Method | Path              | Auth                      | Notes                                  |
| ------ | ----------------- | ------------------------- | -------------------------------------- |
| GET    | `/R`              | any                       | `?limit=&offset=` (default 50, cap 500). |
| GET    | `/R/:id`          | any                       | 404 if unknown.                        |
| POST   | `/R`              | admin + content_editor    | Returns 201 + row.                     |
| PUT    | `/R/:id`          | admin + content_editor    | Omitted fields preserve existing values. |
| DELETE | `/R/:id`          | admin + content_editor    | When approval_required is on, returns `approval` metadata (see § 4.7). |
| POST   | `/R/bulk`         | admin + content_editor    | Body `{ create[], update[], delete[] }`; all operations share one batch_id. |

Dynasty body:
```json
{
  "name": "Tang",
  "start_year": 618,
  "end_year": 907,
  "description": "…",
  "geometry": { "type": "Polygon", "coordinates": [[…]] }
}
```

The `geometry` field on any write is run through `internal/validation`
and rejected with 400 if it exceeds 10 000 vertices or is self-intersecting.

### 4.4 Search

| Method | Path                | Auth              | Query params                                                  |
| ------ | ------------------- | ----------------- | ------------------------------------------------------------- |
| GET    | `/search`           | any               | `q, author_id, dynasty_id, tag_id, meter_id, snippet, highlight, syn, cjk, limit, offset` |
| GET    | `/search/suggest`   | any               | `q, limit`                                                    |
| POST   | `/search/reindex`   | administrator     | _empty_ — rebuilds the inverted index                         |

`/search` response:
```json
{
  "hits": [
    { "poem_id": 1, "title": "Quiet Night", "score": 1.23,
      "matched_fields": ["title", "content"],
      "title_highlighted": "<mark>Quiet</mark> Night", "first_line": "…", "snippet": "…" }
  ],
  "count": 42,
  "did_you_mean": [{ "term": "bright", "distance": 1, "source": "synonym" }],
  "query": "night",
  "options": { "highlight": true, "syn": false, "cjk": false }
}
```

### 4.5 Content packs (offline archive)

| Method | Path                                 | Auth | Notes                                        |
| ------ | ------------------------------------ | ---- | -------------------------------------------- |
| GET    | `/content-packs/current`             | any  | Full JSON archive (may be large).            |
| HEAD   | `/content-packs/current`             | any  | Returns `Content-Length`, `ETag`, `Accept-Ranges: bytes`. |

The GET handler uses `http.ServeContent` — it honours **Range** requests,
**ETag** validation, and **If-None-Match** 304 responses. Clients are
encouraged to use the frontend `offline/download.js` helper which keys
state to the ETag so interrupted downloads resume from the last chunk.

### 4.6 Reviews — `/reviews`

| Method | Path                        | Auth                          | Notes                                    |
| ------ | --------------------------- | ----------------------------- | ---------------------------------------- |
| GET    | `/reviews`                  | any                           | `?poem_id=&user_id=&status=&limit=&offset=`. |
| GET    | `/reviews/:id`              | any                           | 404 on unknown id, 400 on bad id format. |
| POST   | `/reviews`                  | any                           | Body `{ poem_id, rating_*, title, content }`. |
| PUT    | `/reviews/:id`              | owner or administrator        | **Object-level auth**: non-owners get 403. |
| DELETE | `/reviews/:id`              | owner or administrator        | Same rule.                               |
| POST   | `/reviews/:id/moderate`     | administrator + reviewer      | Body `{ status: "approved" \| "rejected" \| "hidden" \| "pending" }`. |

`rating_accuracy`, `rating_readability`, `rating_value` must each be an
integer 1..5. The overall `rating` field is the rounded average, computed
server-side. A fresh review starts with `status="pending"`.

### 4.7 Approvals — `/approvals`

When `system_settings.approval_required = 'true'`, deletions and bulk
operations carry an `approval` metadata block in their 200 response:

```json
{ "deleted": 42, "approval": { "status": "pending", "batch_id": "…", "window_hours": 48 } }
```

Admin-only endpoints for resolving pending batches:

| Method | Path                                   | Auth           | Action                                        |
| ------ | -------------------------------------- | -------------- | --------------------------------------------- |
| GET    | `/approvals`                           | administrator  | List all pending batches.                     |
| POST   | `/approvals/:batch_id/approve`         | administrator  | Seals the batch; changes remain in effect.    |
| POST   | `/approvals/:batch_id/reject`          | administrator  | Reverts every entry in the batch immediately. |

Unknown batch id → 404. If no action is taken within 48 hours, the
background scheduler in `internal/approval/engine.go` auto-reverts the
batch (same as a manual reject).

### 4.8 Revisions — `/revisions` (30-day restore)

Broader than approvals: any audit entry within the 30-day retention
window can be restored by an administrator — not limited to the pending-
approval workflow.

| Method | Path                                 | Auth           | Notes                                                          |
| ------ | ------------------------------------ | -------------- | -------------------------------------------------------------- |
| GET    | `/revisions`                         | administrator  | Required: `entity_type`, `entity_id`; optional: `limit`, `offset`. |
| GET    | `/revisions/supported-entities`      | administrator  | Returns `{ items: [...], retention_days: 30 }`.                |
| POST   | `/revisions/:id/restore`             | administrator  | Applies the `before` snapshot of the chosen revision.          |

Restore semantics:

- **`action=create` revision** → row is deleted.
- **`action=update` revision** → row is set back to the `before` state.
- **`action=delete` revision** → row is re-inserted from `before`.
- Each restore writes a fresh `action=restore` audit entry so the
  operation is itself auditable.

Error cases: 404 unknown id, 400 unsupported entity_type, 409 pending
approval, 410 beyond the 30-day retention window.

### 4.9 Settings — `/settings`

| Method | Path                  | Auth           | Body               | Notes                                  |
| ------ | --------------------- | -------------- | ------------------ | -------------------------------------- |
| GET    | `/settings/approval`  | any            | —                  | Returns `{ approval_required: bool }`. |
| PUT    | `/settings/approval`  | administrator  | `{ enabled: bool }`| Toggles the approval gate globally.    |

### 4.10 Pricing — quote engine

| Method | Path             | Auth | Body                                 | Notes                                               |
| ------ | ---------------- | ---- | ------------------------------------ | --------------------------------------------------- |
| POST   | `/pricing/quote` | any  | `QuoteRequest` (see below)           | Returns a computed `QuoteResult` with applied/rejected discounts. |

`QuoteRequest`:
```json
{
  "user_id": 12,
  "items": [
    { "sku": "poem-1", "price": 10.0, "quantity": 2, "member_priced": false }
  ],
  "coupon_code": "SPRING10",
  "campaign_id": 7,
  "group_size": 5,
  "at": "2026-04-24T12:00:00Z"
}
```

**Security gate.** `user_id` is not trusted from the client. The server
always derives the caller's user from the session; only
`administrator` and `marketing_manager` may override `user_id` for
back-office quoting. Any other caller sending a foreign `user_id`
receives `403`.

`QuoteResult` highlights:
- `total_discount` is capped at 40 % of the discount-eligible subtotal.
- Items with `member_priced=true` are excluded from the discount base.
- `applied` carries each discount's `type`, `name`/`code`, `kind`, `value`
  and `amount`. `rejected` carries `reason` strings when a rule did not fire.

### 4.11 Pricing management — CRUD

Write RBAC: `administrator` + `marketing_manager`. Read: any authenticated
user.

Four resources, each with list/get/create/update/delete:

| Prefix              | Description                                       |
| ------------------- | ------------------------------------------------- |
| `/campaigns`        | Marketing campaigns (standard / flash_sale / group_buy). |
| `/coupons`          | Redeemable coupon codes (UNIQUE `code`).          |
| `/pricing-rules`    | Rule-engine rows (percentage / fixed / tiered / bundle). |
| `/member-tiers`     | Membership tier catalog (UNIQUE `name`, UNIQUE `level`). |

For each prefix `P`:

| Method | Path        | Auth                           |
| ------ | ----------- | ------------------------------ |
| GET    | `P`         | any                            |
| GET    | `P/:id`     | any                            |
| POST   | `P`         | admin + marketing_manager      |
| PUT    | `P/:id`     | admin + marketing_manager      |
| DELETE | `P/:id`     | admin + marketing_manager      |

Validation rules enforced on writes:

- Percentage discount_value must be 0..100.
- Fixed discount_value must be ≥ 0.
- `starts_at` must be `≤` `ends_at` when both provided.
- `campaign_type=group_buy` requires `min_group_size >= 2`.
- `coupons.code` and `member_tiers.name` / `level` are UNIQUE — duplicates
  return **409 Conflict**.
- `coupons.campaign_id` must reference an existing campaign — unknown id
  returns **400**.

Every mutation is recorded in `audit_logs` with `entity_type` ∈
{`campaign`, `coupon`, `pricing_rule`, `member_tier`}.

### 4.12 Complaints — `/complaints`

| Method | Path                               | Auth                         | Notes                                            |
| ------ | ---------------------------------- | ---------------------------- | ------------------------------------------------ |
| POST   | `/complaints`                      | any                          | Body `{ subject, target_type, target_id?, notes? }`. Notes are AES-256-GCM encrypted at rest. |
| GET    | `/complaints/mine`                 | any                          | The caller's own complaints only.                |
| GET    | `/complaints`                      | administrator + reviewer     | `?arbitrator_id=&status=&limit=&offset=`.        |
| GET    | `/complaints/:id`                  | administrator + reviewer     | Decrypted notes included in response.            |
| POST   | `/complaints/:id/assign`           | administrator + reviewer     | Body `{ arbitrator_id }`. **Assignee must be `administrator` or `reviewer`** — any other role returns 400. |
| POST   | `/complaints/:id/resolve`          | administrator + reviewer     | Body `{ arbitration_code, resolution? }`.        |

`target_type` ∈ `{poem, review, user, order, other}`.

Assign response includes the verified arbitrator role:
```json
{ "id": 7, "arbitrator_id": 3, "arbitrator_role": "reviewer" }
```

### 4.13 Arbitration — `/arbitration`

| Method | Path                     | Auth | Notes                                         |
| ------ | ------------------------ | ---- | --------------------------------------------- |
| GET    | `/arbitration/statuses`  | any  | Enumerates codes used by `/complaints/:id/resolve`. |

Seeded codes: `submitted`, `under_review`, `awaiting_evidence`,
`escalated`, `resolved_upheld`, `resolved_rejected`, `withdrawn`.

### 4.14 Crawler — `/crawl`

All routes gated `administrator + crawler_operator`.

| Method | Path                               | Notes                                                   |
| ------ | ---------------------------------- | ------------------------------------------------------- |
| GET    | `/crawl/nodes`                     | Registered workers + heartbeat state.                   |
| GET    | `/crawl/jobs`                      | `?status=&limit=&offset=`.                              |
| GET    | `/crawl/jobs/:id`                  | Single job details including checkpoint.                |
| GET    | `/crawl/jobs/:id/metrics`          | Per-job performance counters.                           |
| GET    | `/crawl/jobs/:id/logs`             | Structured log records emitted by the worker.           |
| POST   | `/crawl/jobs`                      | Body `{ job_name, source_url, config?, priority?, max_attempts?, daily_quota?, scheduled_at? }`. |
| POST   | `/crawl/jobs/:id/pause`            | Transitions to `paused` (409 if not running).           |
| POST   | `/crawl/jobs/:id/resume`           | Transitions to `queued`.                                |
| POST   | `/crawl/jobs/:id/cancel`           | Terminal state `cancelled`.                             |
| POST   | `/crawl/jobs/:id/reset`            | Resets `pages_fetched`, `pages_fetched_today`, `attempts`, `checkpoint`. |

**Per-day quota.** The worker loop calls `CheckAndRollover` before every
fetch, resetting `pages_fetched_today` to 0 when the calendar date (UTC)
changes. A job whose `pages_fetched_today >= daily_quota` is paused by
the worker with `last_error = "daily_quota reached (<N>)"`. The next UTC
midnight the counter resets and a Resume call puts the job back into
service. `daily_quota=0` means "no cap".

### 4.15 Monitoring — `/monitoring`

Gated `administrator` only.

| Method | Path                              | Notes                                              |
| ------ | --------------------------------- | -------------------------------------------------- |
| GET    | `/monitoring/metrics`             | `?limit=&offset=` — request/runtime samples.       |
| GET    | `/monitoring/metrics/summary`     | Last-minute aggregate per metric name.             |
| GET    | `/monitoring/crashes`             | Panics captured by the Gin recovery middleware.    |
| GET    | `/monitoring/crashes/:id`         | Single crash with stack trace + context.           |

Crash endpoint never leaks filesystem errors — unreadable reports log
server-side and respond with 404 or 500 depending on the failure mode.

### 4.16 Audit — `/audit-logs`

| Method | Path           | Auth          | Notes                                                     |
| ------ | -------------- | ------------- | --------------------------------------------------------- |
| GET    | `/audit-logs`  | administrator | `?entity_type=&entity_id=&actor_id=&action=&limit=&offset=`. Includes `before_json` and `after_json` for full diff. |

Rows expire 30 days after creation; expired rows are no longer listed or
restorable (see § 4.8).

---

## 5. Cheat sheet — endpoints by role

| Role                | Must have                                              | Cannot have                                              |
| ------------------- | ------------------------------------------------------ | -------------------------------------------------------- |
| `administrator`     | everything                                             | —                                                        |
| `content_editor`    | content CRUD, own reviews, own complaints              | `/audit-logs`, `/monitoring/*`, `/approvals`, `/revisions`, pricing mgmt writes, `/crawl/*` writes |
| `reviewer`          | `/reviews/:id/moderate`, `/complaints` (list/get/assign/resolve), own reviews | content writes, pricing mgmt writes, `/crawl/*` writes, `/audit-logs` |
| `marketing_manager` | pricing mgmt (campaigns/coupons/rules/tiers) CRUD, `/pricing/quote` with arbitrary `user_id` | content writes, `/crawl/*`, `/audit-logs`, `/revisions`  |
| `crawler_operator`  | `/crawl/*`                                             | content writes, pricing mgmt writes, `/audit-logs`, `/revisions` |
| `member`            | public search, `GET /content-packs/current` and `/member-tiers`, own reviews/complaints, `/auth/me` | every `console` page and every staff endpoint; role returned 403 on attempt |

---

## 6. Conformance

Every route in this document is exercised by the tests under `API_tests/`
with at least one positive and one negative case. The role matrix is
pinned by `API_tests/rolematrix_test.go`. Any change to RBAC or routing
must update this spec in the same commit as the code change.
