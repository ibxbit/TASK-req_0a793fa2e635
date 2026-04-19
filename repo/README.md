**Project type: fullstack (Go/Gin backend + Vue 3 frontend + MySQL 8).**

# Helios — Cultural Content & Operations Management System

Offline-first, locally-deployed platform for managing cultural poems, authors,
reviews, complaints, pricing campaigns, and a multi-node crawler — all served
through a Vue.js public search interface and an RBAC-gated admin console.

- **Backend:** Go 1.22 + Gin
- **Frontend:** Vue 3 + Vite + Pinia + vue-router
- **Database:** MySQL 8.0
- **Deployment:** Docker Compose (single command)

No third-party hosted services are required at runtime. MySQL, the backend
API, and the frontend nginx all run locally on the `helios_net` bridge network.

---

## 1. Start

```bash
docker-compose up
```

> The literal command above is the required entry point. It also works with
> the v2 plugin syntax (`docker compose up`); `run_tests.sh` auto-detects
> which of the two is available. The host machine only needs Docker — every
> runtime component (backend, frontend, MySQL) runs in containers and pulls
> its own dependencies at image build time. No host-level `npm install`,
> `pip install`, or `apt-get install` is ever required.

On first boot (~60–90 s) the images build, `db/init.sql` creates all tables
and seeds the six RBAC roles, and the backend generates its AES-256-GCM key
at `/data/helios-crypto.key` inside the `helios_backend_data` volume. The
backend also seeds one demo user per role on first boot (see § 2.1). Nothing
else needs to be edited — `.env` ships with all variables set.

Subsequent runs skip the build and reuse the MySQL and backend data volumes.

To stop: `Ctrl+C`, then `docker-compose down` (keep data) or
`docker-compose down -v` (wipe data).

---

## 2. Services

| Service  | URL                            | Purpose                                                   |
| -------- | ------------------------------ | --------------------------------------------------------- |
| Frontend | <http://localhost:5173>        | Vue app: public search + `#/console` internal admin panel |
| Backend  | <http://localhost:8080/api/v1> | REST API                                                  |
| MySQL    | `localhost:3306`               | InnoDB, utf8mb4, port exposed for debugging               |

Health probe: `curl http://localhost:8080/api/v1/health` →
`{"status":"ok","db":"up"}`.

### 2.1 Demo credentials (all RBAC roles)

The backend seeds one fixture account per role on first boot
(`internal/auth/bootstrap.go`). All six accounts are active and ready to use:

| Role                | Username   | Password      | Typical allowed actions                                                                        |
| ------------------- | ---------- | ------------- | ---------------------------------------------------------------------------------------------- |
| `administrator`     | `admin`    | `admin123`    | Full access — every endpoint, approvals, audit logs, monitoring, settings, revisions, pricing. |
| `content_editor`    | `editor`   | `editor123`   | Create/edit/delete dynasties, authors, poems, excerpts, tags. Owns their own reviews.          |
| `reviewer`          | `reviewer` | `reviewer123` | Moderate reviews. List/assign/resolve complaints.                                              |
| `marketing_manager` | `marketer` | `marketer123` | Full pricing management: campaigns, coupons, rules, member tiers + back-office quoting.        |
| `crawler_operator`  | `crawler`  | `crawler123`  | Create and control crawl jobs (pause / resume / cancel / reset).                               |
| `member`            | `member`   | `member123`   | Regular end-user: search, browse, download content pack, submit own reviews and complaints.    |

The `admin` credentials may be overridden via `.env` (`ADMIN_USERNAME` /
`ADMIN_PASSWORD`). The other five use fixed defaults so API/UI tests stay
deterministic. Credentials are bcrypt-hashed at rest — the plaintext values
above are only used at seed time.

Members are the **regular user persona**. They have no access to the admin
console, but they can sign in, use the public search, submit reviews, file
their own complaints, and download the offline content pack.

### 2.2 Environment variables (`.env`)

| Var                       | Default            | Meaning                                               |
| ------------------------- | ------------------ | ----------------------------------------------------- |
| `MYSQL_*`                 | see `.env`         | Root + app credentials for MySQL                      |
| `APP_PORT`                | `8080`             | Backend HTTP port (host and container)                |
| `WEB_PORT`                | `5173`             | Frontend host port (container serves on 80 via nginx) |
| `ADMIN_USERNAME/PASSWORD` | `admin`/`admin123` | Seeded admin credentials (min 8 chars)                |
| `HELIOS_COOKIE_SECURE`    | `0`                | Set to `1` when serving the app behind HTTPS          |

---

## 3. Verification walkthrough

Every step can be performed through the UI at <http://localhost:5173> or via
`curl` against the API.

### 3.1 Admin walkthrough

1. **Sign in as `admin` / `admin123`.**
2. **Create content.** Console → **Content** → _Dynasties_ tab → create a
   dynasty. The _Poems_ tab lets you add a poem referencing it.
3. **Search.** Return to the **Search** tab. Instant results (300 ms debounce).
   Toggle Highlight / Synonyms / SC↔TC on the search bar.
4. **Pricing.** Console → **Pricing** → fill the quote form. Verify the 40 %
   cap and member-priced exclusion in the response.
5. **Approvals.** Console → **Settings** → turn on _Require approval_, then
   delete a dynasty in **Content** — the response carries a pending batch id.
6. **Audit log.** Console → **Audit Logs** — every mutation appears with
   before / after JSON. Admin only.
7. **Monitoring.** Console → **Monitoring** — runtime gauges every 30 s. Admin only.

### 3.2 Per-role acceptance checks

| Role                | Sign in as | Expected observable behaviour                                                                                                        |
| ------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `content_editor`    | `editor`   | Can POST /dynasties, /authors, /poems, /tags, /excerpts. `GET /audit-logs`, `/revisions`, `/campaigns` (write) return `403`.         |
| `reviewer`          | `reviewer` | `POST /reviews/:id/moderate` succeeds; `GET /complaints`, assign/resolve succeed (assigning to a non-arbitrator returns `400`).      |
| `marketing_manager` | `marketer` | Full pricing CRUD — `/campaigns`, `/coupons`, `/pricing-rules`, `/member-tiers` — plus back-office `POST /pricing/quote`.            |
| `crawler_operator`  | `crawler`  | Create / pause / resume / cancel / reset crawl jobs; read nodes, metrics, logs. Cannot write content, pricing, or read audits.       |
| `administrator`     | `admin`    | All of the above plus `/audit-logs`, `/monitoring/*`, approvals, `/settings/approval`, `/revisions` (list & restore).                |
| `member`            | `member`   | Public search + `GET /member-tiers` + own reviews / complaints + `/download` for offline pack. Every console endpoint returns `403`. |

### 3.3 Pricing management flow (Blocker 1)

10. **Sign in as `marketer` / `marketer123`.**
11. Console → **Pricing Management** → _Campaigns_ tab → fill `name`,
    `campaign_type`, `discount_type`, `discount_value` → **Create**.
12. Switch to _Coupons_ — add a code referencing the campaign id.
13. _Pricing Rules_ — add an `all`-scope percentage rule, flip active on/off
    with the `Enable`/`Disable` buttons.
14. _Member Tiers_ — add a tier with `level`, `monthly_price`, `yearly_price`.
15. Validation is enforced: percentage > 100 is rejected, `group_buy` without
    `min_group_size` is rejected, duplicate coupon code returns 409.

### 3.4 Revision restore flow (Blocker 2 — 30-day rollback)

16. **Sign in as `admin`.**
17. Console → **Content** → edit any dynasty — change the name and save.
18. Console → **Revisions** → entity `dynasty` + the dynasty id → **Load**.
19. Click **Restore** on the `update` row. Confirm. The revision list
    refreshes and a fresh `action=restore` row appears. Reading the dynasty
    via the Content page or the API shows it is back to its pre-edit state.
20. Entries beyond 30 days are no longer restorable — the backend returns
    `410 Gone` and the UI shows the error.

### 3.5 Member flow (regular end-user persona)

21. **Sign in as `member` / `member123`.**
22. Search is identical to the admin experience. The top nav shows
    **Search** and **Download** links — there is no **Console** entry.
23. Submit a complaint from the search-results area (target type `other`).
24. Click **Download** → **Start** to fetch the offline content pack.
    The progress bar fills, and clicking **Pause** then **Resume** continues
    the same download from the ETag-keyed cache.

### 3.6 Complaints & offline (end-user flow)

25. **Complaints.** Sign in as any role → Console → **Complaints** → submit.
    As `reviewer` or `admin`, assign / resolve. Assigning to a user whose
    role is not `reviewer`/`administrator` is rejected with `400`.
26. **Offline.** Disable your network. The indicator turns red. Cached
    searches still render; writes land in the queue drawer and drain when
    connectivity returns (server dedupes via `Idempotency-Key`).

---

## 4. Tests

```bash
./run_tests.sh
```

The orchestrator is fully Docker-contained and runs, in order:

1. Go backend unit tests (`golang:1.22-alpine`).
2. Frontend Vitest suite (`node:20-alpine`).
3. Main stack up — `mysql` + `backend` + `frontend` (waits on backend healthcheck).
4. Go API integration tests against the live backend (`golang:1.22-alpine`).
5. Playwright E2E tests against the live frontend (`mcr.microsoft.com/playwright`).

The host only needs Docker. No `npm install`, `pip install`, or `apt-get install`
is executed on the host. Each individual suite is also runnable on its own:

```bash
docker-compose --profile test run --rm test-unit
docker-compose --profile test run --rm test-frontend
docker-compose --profile test run --rm test-api
docker-compose --profile test run --rm test-e2e
```

### 4.1 Backend unit tests — `unit_tests/` + `internal/*_test.go`

`unit_tests/` is a Go package that lives in the repo-root `helios-backend`
module. It imports `internal/...` directly and tests the public API of each
package (pricing engine, search tokenizer + CJK + highlighter, spatial
validator, auth password + lockout, AES-256-GCM crypto, crawler host
limiter). Package-private tests remain next to the code they cover under
`internal/<pkg>/<pkg>_test.go`.

Both trees are exercised by the `test-unit` compose service:
`go test ./internal/... ./unit_tests/...`.

### 4.2 Backend API tests — `API_tests/`

`API_tests/` is its own Go module. Tests use `net/http` + `net/http/cookiejar`
to drive real requests through the backend. `TestMain` waits for the backend
healthcheck before any test runs.

Files:

| File                        | Covers                                                            |
| --------------------------- | ----------------------------------------------------------------- |
| `health_test.go`            | `GET /health`                                                     |
| `auth_test.go`              | login / me / logout, bad password, malformed body                 |
| `rbac_test.go`              | 401 on anonymous reads and writes; admin 200 on admin-only        |
| `rolematrix_test.go`        | Full 6-role × representative-endpoint permission matrix           |
| `content_test.go`           | Dynasty CRUD, bulk create, geometry rejection, poem FK            |
| `authors_test.go`           | Author CRUD + bulk, role-scoped write                             |
| `tags_test.go`              | Tag CRUD + bulk                                                   |
| `poems_test.go`             | Poem update + bulk CRUD                                           |
| `excerpts_test.go`          | Excerpt CRUD + bulk, poem filter, annotation_type validation      |
| `reviews_test.go`           | Review CRUD, moderation, owner-only update (object-level auth)    |
| `complaints_test.go`        | Submit, `/mine`, staff list/assign/resolve, encryption round-trip |
| `arbitration_test.go`       | Seeded arbitration_status listing                                 |
| `approvals_test.go`         | Approve and reject batch flows, unknown batch, non-admin denied   |
| `settings_test.go`          | PUT `/settings/approval` + approval metadata on deletes           |
| `crawler_test.go`           | Jobs lifecycle, metrics, logs, transition conflicts, RBAC         |
| `monitoring_test.go`        | Metrics, summary, crashes listing, crash 404/400                  |
| `contentpack_test.go`       | GET and HEAD on `/content-packs/current`, ETag, payload shape     |
| `pricing_test.go`           | 40 % cap, member exclusion, rejected coupon, empty items          |
| `pricing_security_test.go`  | Client-supplied `user_id` is rejected unless role permits it      |
| `pricing_mgmt_test.go`      | Campaign / coupon / rule / tier CRUD + RBAC + validation          |
| `revisions_test.go`         | List, restore create/update/delete, unknown id, forbidden role    |
| `complaints_assign_test.go` | Assign rejects non-staff user_id and unknown user_id              |
| `search_test.go`            | option defaults, highlight flag, suggest, reindex RBAC            |
| `idempotency_test.go`       | same `Idempotency-Key` replayed → same row id                     |
| `errors_test.go`            | 400 / 404 / 405 shapes + `"error"` field                          |

Executed by the `test-api` compose service:
`HELIOS_API_BASE=http://backend:8080/api/v1 go test ./...`.

### 4.3 Frontend unit tests — `frontend/src/**/*.test.js`

Vitest + `@vue/test-utils` drive the frontend tests. `fake-indexeddb` backs
IndexedDB under jsdom so the offline queue and cache can be exercised
without a browser.

| File                                        | Covers                                                                 |
| ------------------------------------------- | ---------------------------------------------------------------------- |
| `src/composables/useAuth.test.js`           | session check, login success / failure, logout cleanup                 |
| `src/composables/useFilters.test.js`        | filter population, idempotency, graceful API failure                   |
| `src/composables/useRbac.test.js`           | role constants, `hasAny` behaviour                                     |
| `src/composables/useSearch.test.js`         | query/options params, online/offline cache fallback, error path        |
| `src/offline/cache.test.js`                 | set / get / TTL expiry / delete / sticky                               |
| `src/offline/queue.test.js`                 | enqueue, offline pause, drain online, idempotency header, 4xx vs 5xx   |
| `src/offline/api.test.js`                   | `apiWrite` online vs offline vs network-error; `apiGet` cache fallback |
| `src/components/HighlightedText.test.js`    | `<mark>` parsing, fallback prop                                        |
| `src/components/LoginForm.test.js`          | submit calls `login`, disables button while busy                       |
| `src/components/NetworkIndicator.test.js`   | reflects `isOnline` state                                              |
| `src/components/SearchBar.test.js`          | query + option binding, loading indicator                              |
| `src/components/FilterPanel.test.js`        | loadFilters on mount, option rendering, clear-all                      |
| `src/components/ResultCard.test.js`         | title/snippet render, highlighted segments, fallbacks                  |
| `src/components/ResultsList.test.js`        | empty hint, cached tag, pager, error region                            |
| `src/components/DidYouMean.test.js`         | renders chips, click rewrites query                                    |
| `src/components/QueueDrawer.test.js`        | toggle, list rows, retry/drop actions                                  |
| `src/pages/SearchPage.test.js`              | composition of search shell                                            |
| `src/App.test.js`                           | loading → login → authenticated state; sign-out action                 |
| `src/router/index.test.js`                  | route table, anonymous guard, admin-only role gating                   |
| `src/console/ConsoleLayout.test.js`         | side nav + header + router-view composition                            |
| `src/console/pages/DashboardPage.test.js`   | health + node stats; admin-only pending-approvals card                 |
| `src/console/pages/SettingsPage.test.js`    | loads toggle, PUTs with Idempotency-Key, error surfacing               |
| `src/console/pages/PricingMgmtPage.test.js` | tab switching, resource loading, create form POST, error banner        |
| `src/console/pages/RevisionsPage.test.js`   | supported-entities load, list query, restore flow, error paths         |
| `src/console/pages/ContentPackPage.test.js` | resumable download start/pause/reset, error surfacing, offline gate    |

All frontend tests run **inside the Docker test profile** — the host does not
need `node` or `npm` installed. See `docker-compose.yml` service `test-frontend`
and `run_tests.sh`.

### 4.4 Fullstack E2E tests — `e2e/tests/*.spec.js`

Playwright drives a real Chromium browser against the running compose stack
(frontend nginx → backend Go → MySQL). No mocks or stubs — every request
lands on a real HTTP endpoint and a real database row.

| File                                      | Covers                                                                         |
| ----------------------------------------- | ------------------------------------------------------------------------------ |
| `e2e/tests/login.spec.js`                 | sign-in form, bad-password error, sign-out returns to login                    |
| `e2e/tests/search.spec.js`                | debounced `/api/v1/search` request on typing, filter toggle forwarding         |
| `e2e/tests/content-mutation.spec.js`      | admin creates a dynasty via UI → verified via API fetch + reviewer route guard |
| `e2e/tests/pricing-mgmt.spec.js`          | marketing manager creates a campaign via UI → verified via API                 |
| `e2e/tests/revisions.spec.js`             | admin restores a prior dynasty revision via UI                                 |
| `e2e/tests/content-pack-download.spec.js` | member starts content pack download, verifies request + offline gating         |

Executed by the `test-e2e` compose service:
`docker-compose --profile test run --rm test-e2e`.

### Coverage map against the spec

| Requirement                                    | Where                                                                                                                                                 |
| ---------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Core business logic & workflows                | `unit_tests/pricing_unit_test.go`, `..._search_...`, `..._validation_...`, `..._auth_...`, `..._crypto_...`, `..._crawler_...`                        |
| State transitions                              | login lockout → unlock, session round-trip, complaint arbitration, crawl job lifecycle                                                                |
| Boundary conditions                            | 10 000-vertex cap, 40 % discount cap, self-intersection, rating 1–5                                                                                   |
| REST endpoint success & failure                | every `API_tests/*_test.go` file                                                                                                                      |
| RBAC enforcement                               | `API_tests/rbac_test.go`, `API_tests/rolematrix_test.go`                                                                                              |
| Object-level authorization                     | `API_tests/reviews_test.go` (owner-only update)                                                                                                       |
| Error responses (HTTP code + JSON envelope)    | `API_tests/errors_test.go`                                                                                                                            |
| Offline semantics (idempotent writes)          | `API_tests/idempotency_test.go`, `frontend/src/offline/queue.test.js`                                                                                 |
| Offline cache fallback                         | `frontend/src/offline/cache.test.js`, `frontend/src/offline/api.test.js`                                                                              |
| Data persistence (MySQL + audit + settings)    | exercised implicitly through the CRUD + RBAC tests                                                                                                    |
| Client-supplied identity spoofing              | `API_tests/pricing_security_test.go`                                                                                                                  |
| Object single-read endpoint (GET /reviews/:id) | `API_tests/reviews_test.go:TestReviews_GetByID` (200/400/404 branches)                                                                                |
| Fullstack browser-driven flows                 | `e2e/tests/login.spec.js`, `search.spec.js`, `content-mutation.spec.js`, `pricing-mgmt.spec.js`, `revisions.spec.js`, `content-pack-download.spec.js` |
| Pricing management CRUD (Blocker 1)            | `API_tests/pricing_mgmt_test.go`, `e2e/tests/pricing-mgmt.spec.js`, `frontend/src/console/pages/PricingMgmtPage.test.js`                              |
| Revision restore within 30 days (Blocker 2)    | `API_tests/revisions_test.go`, `internal/handlers/revisions.go` (30-day gate), `e2e/tests/revisions.spec.js`                                          |
| Member (regular user) persona                  | `internal/auth/bootstrap.go` demo fixture + `API_tests/rolematrix_test.go:TestRoleMatrix_MemberCannotAccessConsole`                                   |
| Offline content-pack download (wired into UI)  | `frontend/src/console/pages/ContentPackPage.vue`, `ContentPackPage.test.js`, `e2e/tests/content-pack-download.spec.js`                                |
| Crawler daily quota (per-day partition)        | `internal/crawler/quota.go`, `internal/crawler/quota_test.go`                                                                                         |
| Complaint arbitrator role validation           | `internal/handlers/complaints.go:allowedArbitratorRoles`, `API_tests/complaints_assign_test.go`                                                       |
| Session 30-minute idle timeout                 | `internal/auth/session.go`, `internal/auth/session_timeout_test.go`                                                                                   |

---

## 4a. Endpoint inventory

| Path                                                         | Method(s)                | RBAC                                                            |
| ------------------------------------------------------------ | ------------------------ | --------------------------------------------------------------- | ------ | --------- | ------------------------ |
| `/auth/login` · `/logout` · `/me` · `/register`              | POST / GET               | public login, authed for `me`/`logout`                          |
| `/health`                                                    | GET                      | public                                                          |
| `/search` · `/search/suggest`                                | GET                      | authed                                                          |
| `/content-packs/current`                                     | GET, HEAD (Range + ETag) | authed                                                          |
| `/dynasties` · `/authors` · `/poems` · `/excerpts` · `/tags` | CRUD + `/bulk`           | read: authed; write: admin + content_editor                     |
| `/reviews` (CRUD, `/:id/moderate`)                           | CRUD, POST               | read: authed; write: authed (owner); moderate: admin + reviewer |
| `/complaints` · `/mine` · `/:id/assign` · `/:id/resolve`     | POST, GET                | complainant: authed; list/assign/resolve: admin + reviewer      |
| `/arbitration/statuses`                                      | GET                      | authed                                                          |
| `/approvals` · `/:batch/approve` · `/reject`                 | GET, POST                | admin                                                           |
| **`/revisions`** · `/supported-entities` · `/:id/restore`    | GET, POST                | **admin — 30-day window** (new)                                 |
| `/settings/approval`                                         | GET, PUT                 | read: authed; write: admin                                      |
| `/pricing/quote`                                             | POST                     | authed (user_id spoof-protected)                                |
| **`/campaigns`** · `/:id`                                    | GET, POST, PUT, DELETE   | read: authed; **write: admin + marketing_manager** (new)        |
| **`/coupons`** · `/:id`                                      | GET, POST, PUT, DELETE   | read: authed; **write: admin + marketing_manager** (new)        |
| **`/pricing-rules`** · `/:id`                                | GET, POST, PUT, DELETE   | read: authed; **write: admin + marketing_manager** (new)        |
| **`/member-tiers`** · `/:id`                                 | GET, POST, PUT, DELETE   | read: authed; **write: admin + marketing_manager** (new)        |
| `/crawl/nodes` · `/jobs` · `/:id/pause                       | resume                   | cancel                                                          | reset` | GET, POST | admin + crawler_operator |
| `/monitoring/metrics` · `/summary` · `/crashes` · `/:id`     | GET                      | admin                                                           |
| `/audit-logs`                                                | GET                      | admin                                                           |

New endpoints added in this revision are marked **bold**.

---

## 5. Directory

```
repo/
├── docker-compose.yml              # mysql + backend + frontend + test-unit + test-api + test-frontend + test-e2e
├── Dockerfile.backend              # multi-stage Go build (context = repo root)
├── .env                            # defaults; no manual edits required
├── .dockerignore
├── README.md
├── run_tests.sh                    # unit → stack up → API → E2E → summary
├── go.mod                          # helios-backend module (repo root)
├── go.sum
├── main.go                         # backend entry point
├── internal/
│   ├── auth/                       # bcrypt, sessions, RBAC, lockout, role fixture seed
│   ├── crypto/                     # AES-256-GCM, local key
│   ├── db/                         # MySQL pool
│   ├── settings/                   # key-value runtime settings
│   ├── audit/                      # audit log writer
│   ├── approval/                   # reverter registry + auto-revert scheduler
│   ├── idempotency/                # middleware + sweeper
│   ├── validation/                 # spatial validator (10k vertices, self-cross)
│   ├── search/                     # inverted index, suggest, CJK, highlight
│   ├── pricing/                    # quote engine (40% cap, stacking)
│   ├── crawler/                    # worker + elastic scheduler + host limiter
│   ├── monitoring/                 # runtime sampler + crash logger
│   └── handlers/                   # REST routers (one file per resource)
├── unit_tests/                     # Go tests that import internal/ via public API
├── API_tests/                      # Go module — integration tests over HTTP
├── e2e/                            # Playwright fullstack E2E — runs in Docker via `test-e2e`
│   ├── Dockerfile
│   ├── package.json
│   ├── playwright.config.js
│   └── tests/                      # login, search, content-mutation specs
├── db/
│   └── init.sql                    # schema + seed data
└── frontend/
    ├── Dockerfile                  # build + nginx with /api proxy
    ├── nginx.conf
    ├── package.json                # includes vitest + @vue/test-utils + fake-indexeddb
    ├── vite.config.js              # vitest config + jsdom environment
    ├── index.html
    └── src/
        ├── main.js
        ├── App.vue
        ├── router/
        ├── __tests__/setup.js      # reset IndexedDB + module cache between tests
        ├── pages/SearchPage.vue    # public instant search
        ├── components/             # SearchBar, FilterPanel, ResultCard, …
        │   └── *.test.js           # component unit tests
        ├── composables/            # useAuth, useSearch, useFilters, useRbac
        │   └── *.test.js           # composable unit tests
        ├── offline/                # IndexedDB + queue + cache + download
        │   └── *.test.js           # offline-layer unit tests
        └── console/                # admin pages + sidebar
```

Layered responsibility:

- **Routes** — `internal/handlers/*`
- **Business logic** — `internal/{pricing,search,approval,crawler,auth,…}`
- **Data access** — `internal/db` (pool) + parameterized queries per package

Frontend mirrors the split: **components** render, **composables** hold
state and API logic, **offline/** owns IndexedDB + retries.

---

## 6. Offline compliance

- All service-to-service traffic stays on the `helios_net` docker network.
- The backend binary makes no outbound HTTP except when the crawler executes
  an operator-created job against a URL of the operator's choosing.
- The frontend bundle ships through nginx — no CDN tags in `index.html`.
- Session cookies, AES key, audit log, metrics, crash reports, and offline
  queue all live on local volumes or IndexedDB.
- `/pricing/quote` never trusts a client-supplied `user_id` for member
  eligibility — it derives the caller's user id from the session cookie and
  only lets administrators or marketing_managers override it.
- `/monitoring/crashes/:id` does not leak filesystem errors to clients
  when an on-disk report is unreadable; the backend logs the detail instead.
- Handler DB errors return a redacted 500 response; the raw driver error is
  logged server-side (never included in the JSON envelope).

---

## 7. Delivery checklist

- [x] One-command start: `docker-compose up`
- [x] All ports exposed (`5173`, `8080`, `3306`)
- [x] No private libraries — `go.mod` + `frontend/package.json` declare
      every runtime dependency
- [x] `README.md` documents start, services, verification, and demo
      credentials for every RBAC role
- [x] `run_tests.sh` at repo root drives unit + API Go suites + frontend
      Vitest suite, idempotent
- [x] Unit + API + frontend tests cover core logic, RBAC (all six roles),
      errors, offline semantics, object-level authorization
- [x] Routes / Business Logic / Data Access separated
- [x] UI buttons expose loading (`:disabled="busy"`) states on long-running
      actions
- [x] Full offline compliance verified (no external API at runtime)
