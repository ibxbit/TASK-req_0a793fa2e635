import { test, expect, request } from '@playwright/test'

// Real fullstack pricing management: marketing manager creates a coupon
// via the UI, then we verify persistence through the backend API.

async function login(page, user, pass) {
  await page.goto('/')
  await page.locator('input[autocomplete="username"]').fill(user)
  await page.locator('input[type="password"]').fill(pass)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByText(`${user} ·`)).toBeVisible()
}

test('marketing manager creates a campaign via the UI and it persists', async ({ page, baseURL }) => {
  await login(page, 'marketer', 'marketer123')
  await page.getByRole('link', { name: 'Console', exact: true }).click()
  await page.getByRole('link', { name: 'Pricing Management', exact: true }).click()

  // Campaigns tab is the default.
  const name = 'E2E_Campaign_' + Date.now()
  const inputs = page.locator('form.create input')
  await inputs.first().fill(name) // name
  await page.getByRole('button', { name: 'Create', exact: true }).click()

  // Wait for the created row to show up in the list table.
  await expect(page.locator('table.list tbody tr', { hasText: name })).toBeVisible()

  // Persistence check via API.
  const ctx = await request.newContext({ baseURL, ignoreHTTPSErrors: true })
  await ctx.post('/api/v1/auth/login', { data: { username: 'marketer', password: 'marketer123' } })
  const listRes = await ctx.get('/api/v1/campaigns?limit=500')
  const json = await listRes.json()
  const row = (json.items || []).find((c) => c.name === name)
  expect(row, 'campaign row should exist in DB').toBeTruthy()

  // Cleanup.
  await ctx.delete(`/api/v1/campaigns/${row.id}`, {
    headers: { 'Idempotency-Key': 'e2e-campaign-cleanup-' + row.id },
  })
  await ctx.dispose()
})

test('reviewer cannot open pricing management', async ({ page }) => {
  await login(page, 'reviewer', 'reviewer123')
  // Side nav hides the link; direct URL is guarded.
  await page.goto('/#/console/pricing-mgmt')
  await expect(page).toHaveURL(/#\/console\/dashboard$/)
})
