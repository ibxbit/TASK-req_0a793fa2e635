import { test, expect, request } from '@playwright/test'

// End-to-end write path: UI → nginx proxy → Go backend → MySQL. After the UI
// mutation we ask the backend directly whether the row persisted, proving
// the full stack is wired up and RBAC is actually enforced.

async function login(page, user = 'admin', pass = 'admin123') {
  await page.goto('/')
  await page.locator('input[autocomplete="username"]').fill(user)
  await page.locator('input[type="password"]').fill(pass)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByText(`${user} ·`)).toBeVisible()
}

test('admin creates a dynasty via the UI and it persists in the DB', async ({ page, baseURL }) => {
  await login(page)
  await page.getByRole('link', { name: 'Console', exact: true }).click()
  // SideNav "Content" link — exact so we don't accidentally match "Console".
  await page.getByRole('link', { name: 'Content', exact: true }).click()

  // Click the Dynasties tab inside ContentPage.
  await page.getByRole('button', { name: 'Dynasties', exact: true }).click()

  const name = 'E2E_' + Date.now()
  // The create form uses placeholder="name" for the dynasty form.
  await page.getByPlaceholder('name').first().fill(name)
  await page.getByRole('button', { name: 'Create', exact: true }).click()

  // The UI list refreshes after POST. Wait for the new row to appear.
  await expect(page.locator('.list li .t', { hasText: name })).toBeVisible()

  // Confirm via a real API call that the row exists in MySQL.
  const ctx = await request.newContext({ baseURL, ignoreHTTPSErrors: true })
  const loginRes = await ctx.post('/api/v1/auth/login', {
    data: { username: 'admin', password: 'admin123' },
  })
  expect(loginRes.ok()).toBeTruthy()
  const listRes = await ctx.get('/api/v1/dynasties?limit=500')
  expect(listRes.ok()).toBeTruthy()
  const json = await listRes.json()
  const row = (json.items || []).find(d => d.name === name)
  expect(row, 'dynasty must exist in DB after UI create').toBeTruthy()

  // Clean up — delete via API so UI state doesn't accumulate test rows.
  await ctx.delete(`/api/v1/dynasties/${row.id}`, {
    headers: { 'Idempotency-Key': 'e2e-cleanup-' + row.id },
  })
  await ctx.dispose()
})

test('reviewer cannot reach admin-only Console pages', async ({ page }) => {
  await login(page, 'reviewer', 'reviewer123')
  // Reviewer's role is not in the `audit` page allowlist — guard should redirect
  // to the console dashboard.
  await page.goto('/#/console/audit')
  await expect(page).toHaveURL(/#\/console\/dashboard$/)
})
