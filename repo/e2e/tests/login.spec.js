import { test, expect } from '@playwright/test'

// Fullstack login flow — real browser → nginx proxy → Go backend → MySQL.
// No mocks; uses the seeded demo admin credentials (see README § 2.1).

test.describe('login flow', () => {
  test('admin can sign in and sees the authenticated shell', async ({ page }) => {
    await page.goto('/')
    // Login form is rendered when the session check returns 401.
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
    await page.locator('input[autocomplete="username"]').fill('admin')
    await page.locator('input[type="password"]').fill('admin123')
    await page.getByRole('button', { name: 'Sign in' }).click()

    // Authenticated shell has the user chip and the nav.
    await expect(page.getByText('admin · administrator')).toBeVisible()
    await expect(page.getByRole('link', { name: 'Console' })).toBeVisible()
    // Network indicator says online when the stack is up.
    await expect(page.locator('.net.online')).toBeVisible()
  })

  test('bad credentials surface an error without authenticating', async ({ page }) => {
    await page.goto('/')
    await page.locator('input[autocomplete="username"]').fill('admin')
    await page.locator('input[type="password"]').fill('definitely-wrong')
    await page.getByRole('button', { name: 'Sign in' }).click()

    await expect(page.locator('.login .err')).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
  })

  test('sign-out returns to login form', async ({ page }) => {
    await page.goto('/')
    await page.locator('input[autocomplete="username"]').fill('admin')
    await page.locator('input[type="password"]').fill('admin123')
    await page.getByRole('button', { name: 'Sign in' }).click()
    await expect(page.getByText('admin · administrator')).toBeVisible()
    await page.getByRole('button', { name: 'Sign out' }).click()
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
  })
})
