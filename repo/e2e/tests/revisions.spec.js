import { test, expect, request } from '@playwright/test'

// Revision restore end-to-end: admin creates a dynasty via API, edits it,
// then uses the UI to find the update revision and restore to the
// pre-edit state. Verified by reading the dynasty row back through the API.

async function login(page, user = 'admin', pass = 'admin123') {
  await page.goto('/')
  await page.locator('input[autocomplete="username"]').fill(user)
  await page.locator('input[type="password"]').fill(pass)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByText(`${user} ·`)).toBeVisible()
}

test('admin restores a prior dynasty revision through the UI', async ({ page, baseURL }) => {
  const name0 = 'RevE2E_' + Date.now()
  const ctx = await request.newContext({ baseURL, ignoreHTTPSErrors: true })
  await ctx.post('/api/v1/auth/login', { data: { username: 'admin', password: 'admin123' } })
  const createRes = await ctx.post('/api/v1/dynasties', {
    data: { name: name0, start_year: 1000 },
    headers: { 'Idempotency-Key': 'e2e-rev-create-' + Date.now() },
  })
  const created = await createRes.json()
  const id = created.id
  // Edit so there's an `update` audit row to restore from.
  await ctx.put(`/api/v1/dynasties/${id}`, {
    data: { name: name0 + '_EDITED', start_year: 1200 },
    headers: { 'Idempotency-Key': 'e2e-rev-edit-' + Date.now() },
  })

  await login(page)
  await page.getByRole('link', { name: 'Console', exact: true }).click()
  await page.getByRole('link', { name: 'Revisions', exact: true }).click()
  // Page auto-picked 'dynasty' as the first supported entity — good.
  await page.locator('input[type="number"]').fill(String(id))
  await page.getByRole('button', { name: 'Load' }).click()

  const restoreBtn = page.locator('table.list tbody tr', { hasText: 'update' }).first()
    .locator('button.restore')
  page.once('dialog', dialog => dialog.accept())
  await restoreBtn.click()
  await expect(page.locator('[data-test="rev-ok"]')).toBeVisible()

  // Verify the live row is back to the pre-edit name.
  const afterRes = await ctx.get(`/api/v1/dynasties/${id}`)
  const after = await afterRes.json()
  expect(after.name).toBe(name0)

  await ctx.delete(`/api/v1/dynasties/${id}`)
  await ctx.dispose()
})
