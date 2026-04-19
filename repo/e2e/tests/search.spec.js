import { test, expect } from '@playwright/test'

// Fullstack public search flow — typing into the search bar triggers a real
// /api/v1/search request and the UI updates from the response payload.

async function login(page, user = 'admin', pass = 'admin123') {
  await page.goto('/')
  await page.locator('input[autocomplete="username"]').fill(user)
  await page.locator('input[type="password"]').fill(pass)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByText(`${user} ·`)).toBeVisible()
}

test.describe('search flow', () => {
  test('typing issues a real search request and updates the hint bar', async ({ page }) => {
    await login(page)

    // Wait for the initial empty search to settle, then type.
    const input = page.locator('input[type="search"]')
    await expect(input).toBeVisible()

    // Capture the actual /api/v1/search request triggered by the debounced watcher.
    const reqPromise = page.waitForRequest(u => u.url().includes('/api/v1/search'))
    await input.fill('月')
    const req = await reqPromise
    expect(req.url()).toContain('q=%E6%9C%88')

    // The results bar updates with either "No results" or a results count.
    const bar = page.locator('.results .bar').first()
    await expect(bar).toBeVisible()
    await expect(bar).not.toContainText('Start typing to search')
  })

  test('search filters forward through the request (highlight toggle)', async ({ page }) => {
    await login(page)

    // Disable the default highlight toggle and confirm the next request omits it.
    await page.locator('.toggles label').first().locator('input[type="checkbox"]').uncheck()

    const reqPromise = page.waitForRequest(u => u.url().includes('/api/v1/search'))
    await page.locator('input[type="search"]').fill('春')
    const req = await reqPromise
    expect(req.url()).not.toContain('highlight=1')
  })
})
