import { defineConfig, devices } from '@playwright/test'

// Base URL comes from the compose env — when the test-e2e container runs
// inside docker-compose it points at `http://frontend` (the nginx service).
// When run locally against a host-published stack, set E2E_BASE_URL=http://localhost:5173.
const baseURL = process.env.E2E_BASE_URL || 'http://frontend'

export default defineConfig({
  testDir: './tests',
  // One worker keeps the tests deterministic — they mutate shared server state.
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 15_000 },
  reporter: [['list']],
  use: {
    baseURL,
    headless: true,
    ignoreHTTPSErrors: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
})
