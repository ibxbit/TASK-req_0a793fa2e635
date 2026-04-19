#!/usr/bin/env bash
# Root test orchestrator. Idempotent — rerunning produces the same result.
# Everything runs inside containers; the host only needs Docker + docker-compose.
#
#   1. Backend unit tests — `docker-compose --profile test run test-unit`
#      (go test ./internal/... ./unit_tests/... in golang:1.22-alpine).
#   2. Frontend unit tests — `docker-compose --profile test run test-frontend`
#      (vitest in node:20-alpine).
#   3. Brings the main stack up (mysql + backend + frontend) and waits for
#      the backend's healthcheck to report healthy.
#   4. Backend API tests — `docker-compose --profile test run test-api`
#      (go test ./... in API_tests/ against the live backend).
#   5. Fullstack E2E — `docker-compose --profile test run test-e2e`
#      (Playwright against the live frontend at http://frontend).
#
# Exit code: 0 when all four suites pass, non-zero otherwise.

set -u

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

UNIT_RC=0
FE_RC=0
API_RC=0
E2E_RC=0

say() { printf "\n\033[1;34m==>\033[0m %s\n" "$1"; }

# Prefer the v2 plugin when available, but fall back to v1 so the literal
# `docker-compose` command still works in CI systems that only have the
# legacy binary installed.
if docker compose version >/dev/null 2>&1; then
  DC="docker compose"
else
  DC="docker-compose"
fi

# -------- 1. Backend unit tests --------
say "Backend unit tests (go test ./internal/... ./unit_tests/...)"
$DC --profile test run --rm test-unit
UNIT_RC=$?

# -------- 2. Frontend unit tests --------
say "Frontend unit tests (vitest)"
$DC --profile test run --rm test-frontend
FE_RC=$?

# -------- 3. Bring up the stack --------
say "Starting the main stack"
$DC up -d --build mysql backend frontend

say "Waiting for backend to become healthy"
deadline=$((SECONDS + 180))
while true; do
  health=$($DC ps backend --format '{{.Health}}' 2>/dev/null || echo "")
  case "$health" in
    healthy) break ;;
  esac
  if [ "$SECONDS" -ge "$deadline" ]; then
    echo "ERROR: backend did not become healthy within 180s" >&2
    $DC logs --tail=120 backend >&2 || true
    exit 2
  fi
  sleep 3
done
echo "backend is healthy."

# -------- 4. API tests --------
say "API tests (go test ./... in API_tests/)"
$DC --profile test run --rm test-api
API_RC=$?

# -------- 5. Playwright E2E --------
say "Fullstack E2E (Playwright)"
$DC --profile test run --rm test-e2e
E2E_RC=$?

# -------- Summary --------
echo
say "Summary"
printf "  backend unit tests  : %s\n" "$( [ $UNIT_RC -eq 0 ] && echo PASSED || echo FAILED )"
printf "  frontend unit tests : %s\n" "$( [ $FE_RC   -eq 0 ] && echo PASSED || echo FAILED )"
printf "  API tests           : %s\n" "$( [ $API_RC  -eq 0 ] && echo PASSED || echo FAILED )"
printf "  E2E tests           : %s\n" "$( [ $E2E_RC  -eq 0 ] && echo PASSED || echo FAILED )"

if [ "$UNIT_RC" -ne 0 ] || [ "$FE_RC" -ne 0 ] || [ "$API_RC" -ne 0 ] || [ "$E2E_RC" -ne 0 ]; then
  exit 1
fi
exit 0
