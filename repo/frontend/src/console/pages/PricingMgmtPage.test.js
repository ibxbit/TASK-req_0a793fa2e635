import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const api = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}))

vi.mock('../api.js', () => ({
  get: api.get,
  post: api.post,
  put: api.put,
  del: api.del,
}))

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset())
})

describe('PricingMgmtPage.vue', () => {
  it('loads campaigns on mount and renders them', async () => {
    api.get.mockResolvedValueOnce({ items: [
      { id: 1, name: 'Flash', campaign_type: 'flash_sale', discount_type: 'percentage', discount_value: 10, status: 'active' },
    ] })
    const { default: Page } = await import('./PricingMgmtPage.vue')
    const w = mount(Page)
    await flushPromises()
    expect(api.get).toHaveBeenCalledWith('/campaigns', { limit: 200 })
    expect(w.text()).toContain('Flash')
    expect(w.text()).toContain('flash_sale')
  })

  it('switching to a different tab loads its resource', async () => {
    api.get.mockResolvedValueOnce({ items: [] }) // campaigns (initial)
    api.get.mockResolvedValueOnce({ items: [{ id: 5, code: 'X', discount_type: 'percentage', discount_value: 20, used_count: 0, status: 'active' }] })
    const { default: Page } = await import('./PricingMgmtPage.vue')
    const w = mount(Page)
    await flushPromises()
    // Click Coupons tab (second button in .tabs).
    const tabs = w.findAll('.tabs button')
    await tabs[1].trigger('click')
    await flushPromises()
    expect(api.get.mock.calls[1][0]).toBe('/coupons')
    expect(w.text()).toContain('X') // coupon code rendered
  })

  it('submitting the create form POSTs a sanitized payload', async () => {
    api.get.mockResolvedValue({ items: [] })
    api.post.mockResolvedValueOnce({ id: 42 })
    const { default: Page } = await import('./PricingMgmtPage.vue')
    const w = mount(Page)
    await flushPromises()

    const inputs = w.findAll('.create input')
    // First input is name; fill only that so the defaults flow through.
    await inputs[0].setValue('My Campaign')
    await w.find('.create').trigger('submit.prevent')
    await flushPromises()
    expect(api.post).toHaveBeenCalled()
    const [url, body] = api.post.mock.calls[0]
    expect(url).toBe('/campaigns')
    expect(body.name).toBe('My Campaign')
    expect(body.campaign_type).toBe('standard')
  })

  it('surfaces backend errors into the error banner', async () => {
    api.get.mockResolvedValueOnce({ items: [] })
    api.post.mockRejectedValueOnce({ response: { data: { error: 'name required' } } })
    const { default: Page } = await import('./PricingMgmtPage.vue')
    const w = mount(Page)
    await flushPromises()
    await w.find('.create').trigger('submit.prevent')
    await flushPromises()
    expect(w.find('[data-test="pm-error"]').text()).toBe('name required')
  })
})
