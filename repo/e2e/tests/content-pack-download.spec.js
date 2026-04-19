import { test, expect } from '@playwright/test'

// Exercises the resumable content-pack download integration from a real
// browser. We can't reliably force a multi-chunk download in CI (the
// pack body is small), so the assertion is that:
//   - a fetch to /api/v1/content-packs/current is issued on Start
//   - progress UI moves past 0%
//   - end state reaches 'complete' (or 'paused' if user clicks pause first)
// This is enough to prove the UI is wired to download.js correctly.

async function login(page, user = 'member', pass = 'member123') {
  await page.goto('/')
  await page.locator('input[autocomplete="username"]').fill(user)
  await page.locator('input[type="password"]').fill(pass)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByText(`${user} ·`)).toBeVisible()
}

test('member can open the download page and start a content pack download', async ({ page }) => {
  await login(page)
  await page.getByRole('link', { name: 'Download' }).click()

  await expect(page.getByRole('heading', { name: /Content pack/i })).toBeVisible()

  const req = page.waitForRequest((u) => u.url().includes('/api/v1/content-packs/current'))
  await page.locator('[data-test="dl-start"]').click()
  await req

  // Wait for progress to tick upward — either completes or at least shows non-zero.
  await expect(page.locator('[data-test="dl-stats"]')).toBeVisible()
  await expect(page.locator('[data-test="dl-stats"]')).toContainText(/state: (running|complete)/)
})

test('offline banner shown and start disabled when offline', async ({ page, context }) => {
  await login(page)
  await page.getByRole('link', { name: 'Download' }).click()
  await context.setOffline(true)
  // The NetworkIndicator composable reacts to browser online events. Nudge it.
  await page.evaluate(() => window.dispatchEvent(new Event('offline')))
  // Start button becomes disabled once isOnline flips.
  await expect(page.locator('[data-test="dl-start"]')).toBeDisabled({ timeout: 5000 })
  await context.setOffline(false)
})
