# Advanced Feature Flows

This document provides static evidence for four production-critical flows: offline assurance, resumable downloads, local performance monitoring and crash reporting, and revision rollback within a 30-day window.

---

## 1. Offline Assurance

### Client-side cache (read path)

`frontend/src/offline/cache.js` — IndexedDB-backed key/value cache with per-entry TTL.

| Function | Behaviour |
|---|---|
| `cacheSet(key, value, ttlMs)` | Stores entry with `expires_at = Date.now() + ttlMs`. `ttlMs=0` means never-expires. |
| `cacheGet(key)` | Returns `null` and deletes the entry if `expires_at < Date.now()`. |
| `cacheDelete(key)` | Explicit eviction. |

`frontend/src/offline/api.js` — `apiGet(url, { cacheKey, ttlMs, params })` reads from the cache first; falls through to a network `fetch` on miss, or serves the stale cache entry when the network request fails (`fromCache: true` in the return value).

**Test evidence**: `frontend/src/console/api.test.js` — the `get()` function is asserted to use axios directly and not the write queue. `MemberComplaintsPage.test.js` — the `fromCache: true` path asserts that a `[data-test="cache-note"]` banner is shown.

### Persistent retry queue (write path)

`frontend/src/offline/queue.js` — IndexedDB `queue` store. Each entry carries:

- `id` — `crypto.randomUUID()` used as idempotency key
- `method`, `url`, `body`
- `kind` — `'edit'` | `'review'` | `'complaint'` (controls QueueDrawer grouping)
- `status` — `pending` | `in_flight` | `done` | `failed`
- `retries` — capped at 5; exponential backoff (1 s × 2^n)
- `idempotency_key` — same value as `id`, attached as `Idempotency-Key` header on every flush attempt

`processQueue()` runs on `onNetworkChange` (network restore) and on each new `enqueue()` call. Entries transition `pending → in_flight → done | failed`. Failed entries with `retries < 5` are re-queued for the next flush cycle.

**Test evidence**: `frontend/src/offline/queue.test.js` covers enqueue, processQueue drain, retry increment, max-retry halt, and idempotency key attachment.

### Idempotency for writes

`frontend/src/offline/api.js` — `apiWrite({ method, url, body, kind })` calls `enqueue()` when offline (or on network error), which generates a UUID that is reused as `Idempotency-Key` on every retry attempt. When online the key is sent immediately on the first attempt.

The backend honours `Idempotency-Key` on `POST /reviews`, `POST /complaints`, and all admin mutations — duplicate requests within 24 h return the original response without re-executing side effects (`API_tests/idempotency_test.go`).

**Test evidence**:
- `frontend/src/console/api.test.js` — asserts `post()`, `put()`, `del()` all call `apiWrite` with `kind:'edit'` and the full `/api/v1` URL prefix.
- `API_tests/member_submit_test.go` — `TestMember_ReviewIdempotencyKey` and `TestMember_ComplaintIdempotencyKey` confirm the backend deduplicates repeated submissions.

---

## 2. Resumable Downloads

### Implementation

`frontend/src/offline/download.js` — `resumableDownload({ url, key, onProgress, signal })`.

**Algorithm**:
1. Read existing partial state from IndexedDB (`downloads` store, keyed by `key`).
2. `HEAD` the URL to get `Content-Length`, `ETag`, and `Accept-Ranges`.
3. If the stored `etag` differs from the server ETag → discard stale state and restart from byte 0.
4. If the server does not return `Accept-Ranges: bytes` → fall back to a plain `GET` for the full body.
5. Loop: `GET bytes=<downloaded>-<downloaded+1MiB-1>` with `If-Match: <etag>`.  
   After each 1 MiB chunk, append to `state.chunks`, increment `state.downloaded`, and `put()` the updated state to IndexedDB.
6. On `AbortSignal` abort — partial progress remains in IndexedDB; the next call to `resumableDownload` with the same `key` resumes from `state.downloaded`.
7. Returns `Uint8Array` of concatenated chunks.

`clearDownload(key)` removes a completed (or abandoned) entry from IndexedDB.

### Test evidence

`frontend/src/offline/download.test.js` (added in this session):

| Test | Coverage |
|---|---|
| HEAD + Range GET single chunk | Happy path — result bytes correct, state persisted |
| Resume from partial state | Range header starts at `state.downloaded`, not 0 |
| ETag mismatch invalidates state | Range restarts at byte 0, fresh content returned |
| AbortSignal mid-download | Rejects with `AbortError`, partial state still in IndexedDB |
| No `Accept-Ranges` → full GET | Fallback path, no `Range` header on second fetch |
| `clearDownload` | Calls `del()` on the `downloads` store |
| `onProgress` callback | Fires at least once per chunk with `{ downloaded, total }` |
| HEAD failure | Throws, no IndexedDB write |

---

## 3. Local Performance Monitoring and Crash Report Storage

### Backend implementation

**Performance metrics** (`internal/handlers/monitoring.go`, `internal/db/schema.sql`):

- `performance_metrics` table: `id`, `service`, `metric_name`, `metric_value`, `unit`, `tags` (JSON), `recorded_at`.
- `GET /monitoring/metrics` — paginated list, filterable by `?name=` and `?since=` (RFC 3339).
- `GET /monitoring/metrics/summary` — one row per `metric_name` (latest value, `tags IS NULL` rows only).

**Crash reports** (`internal/handlers/monitoring.go`, `internal/monitoring/`):

- `crash_reports` table: `id`, `service`, `environment`, `error_type`, `error_message`, `stack_trace`, `context` (JSON), `occurred_at`, `resolved`.
- `GET /monitoring/crashes` — paginated list; response includes `crash_dir` field (value of `HELIOS_CRASH_DIR` env var) so operators know where disk files are stored.
- `GET /monitoring/crashes/:id` — full detail including `stack_trace`; if `context.disk_path` is set, `monitoring.ReadReport(diskPath)` reads the on-disk copy and adds it as `disk_copy` in the response. Read errors are logged server-side only (not exposed to the client).

All monitoring endpoints are gated behind `auth.AuthRequired()` + `auth.RequireRole("administrator")`.

### Test evidence

`API_tests/monitoring_test.go`:

| Test | Coverage |
|---|---|
| `TestMonitoring_RequiresAdmin` | reviewer gets 403 on metrics, summary, crashes |
| `TestMonitoring_AnonBlocked` | unauthenticated gets 401 on all four endpoints |
| `TestMonitoring_NonAdminRolesBlocked` | editor gets 403 on metrics, summary, crashes |
| `TestMonitoring_MetricsEnvelope` | admin gets 200, response has `items` + echoed `limit` |
| `TestMonitoring_MetricsSummary` | admin gets 200, response has `items` |
| `TestMonitoring_MetricsFilterByName` | `?name=` filter returns 200 with `items` array |
| `TestMonitoring_MetricsFilterBySince` | `?since=` with far-future date returns empty `items` |
| `TestMonitoring_MetricsEnvelopeContainsOffsetField` | pagination envelope has `offset` field |
| `TestMonitoring_CrashesList` | admin gets 200, response has `items` + `crash_dir` |
| `TestMonitoring_CrashNotFoundClean` | unknown id returns 404 with `error` field |
| `TestMonitoring_CrashInvalidIDReturns400` | non-numeric id returns 400 with `error` field |

---

## 4. Rollback to Any Prior Revision within 30 Days

### Backend implementation

`internal/handlers/revisions.go`:

- `RetentionDays = 30` — all audit rows get `expires_at = created_at + 30 days` on INSERT.
- `GET /revisions?entity_type=&entity_id=` — returns only rows where `expires_at > NOW()`; expired rows are invisible to the caller.
- `POST /revisions/:id/restore` — enforces the retention window with a dual guard:
  1. `expires_at.Before(time.Now())` → **410 Gone** ("revision has expired")
  2. `created_at.Before(time.Now().Add(-RetentionDays * 24 * time.Hour))` → **410 Gone** (defence in depth)
  - Pending-approval rows → **409 Conflict**
  - Unknown id → **404 Not Found**
  - In-window row → calls `approval.RestoreRevision(tx, entityType, action, before, after)`, writes a `restore` audit entry, returns **200 OK** with `{ entity_type, entity_id, action, restored_revision_id }`.

`internal/approval/revert.go` — dispatcher for nine entity types:

| Entity | Reverter |
|---|---|
| dynasty | `revertDynasty` |
| author | `revertAuthor` |
| poem | `revertPoem` |
| excerpt | `revertExcerpt` |
| tag | `revertTag` |
| campaign | `revertCampaign` |
| coupon | `revertCoupon` |
| pricing_rule | `revertPricingRule` |
| member_tier | `revertMemberTier` |

Each reverter handles three actions:
- `create` → DELETE (undo the creation)
- `update` → UPDATE (restore `before` snapshot)
- `delete` → INSERT (re-insert `before` snapshot)

`GET /revisions/supported-entities` returns the full list of auditable types and `retention_days: 30`.

### Test evidence

`API_tests/revisions_test.go`:

| Test | Coverage |
|---|---|
| `TestRevisions_AnonRejected` | unauthenticated → 401 |
| `TestRevisions_NonAdminForbidden` | reviewer → 403 |
| `TestRevisions_ListAndRestore_UpdateRoundTrip` | create → update → list → restore → GET verifies rollback |
| `TestRevisions_RestoreCreateRemovesRow` | restore of a "create" revision deletes the row |
| `TestRevisions_RestoreDeleteReinsertsRow` | restore of a "delete" revision re-inserts the row |
| `TestRevisions_RestoreUnknownID` | 404 with error field |
| `TestRevisions_ListRequiresEntityParams` | 400 on missing / malformed params |
| `TestRevisions_PendingApprovalConflictOnRestore` | 409 when revision is pending approval |
| `TestRevisions_SupportedEntities` | all 9 entity types listed, `retention_days = 30` |
| `TestRevisions_ExpiredRevision_Returns410` | direct DB INSERT with `expires_at` 31 days ago → 410 |
| `TestRevisions_InWindowRestore_Returns200` | freshly created revision → 200 |
| `TestRevisions_Campaign_{Update,Delete,Create}Restore` | full round-trip for campaign |
| `TestRevisions_Coupon_{Update,Delete,Create}Restore` | full round-trip for coupon |
| `TestRevisions_PricingRule_{Update,Delete,Create}Restore` | full round-trip for pricing_rule |
| `TestRevisions_MemberTier_{Update,Delete,Create}Restore` | full round-trip for member_tier |
