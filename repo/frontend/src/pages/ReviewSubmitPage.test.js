import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

// Only vi.fn() instances inside vi.hoisted() — imported symbols (ref) are not
// yet available when the hoisted factory runs, so refs live at module scope.
const { mockSubmitReview, mockReset } = vi.hoisted(() => ({
  mockSubmitReview: vi.fn(),
  mockReset: vi.fn(),
}))

// Reactive state at module scope: imports are resolved before this runs.
// The vi.mock() factory below closes over reviewsState and is called lazily
// (when the component first imports useReviews), so reviewsState is ready.
const reviewsState = {
  submitting: ref(false),
  submitError: ref(''),
  submitResult: ref(null),
  submitReview: mockSubmitReview,
  reset: mockReset,
}

vi.mock('../composables/useReviews.js', () => ({
  useReviews: () => reviewsState,
}))

beforeEach(() => {
  reviewsState.submitting.value = false
  reviewsState.submitError.value = ''
  reviewsState.submitResult.value = null
  mockSubmitReview.mockReset()
  mockReset.mockReset()
})

describe('ReviewSubmitPage.vue', () => {
  it('renders the form fields', async () => {
    const { default: Page } = await import('./ReviewSubmitPage.vue')
    const w = mount(Page)
    expect(w.find('#poem-id').exists()).toBe(true)
    expect(w.find('#r-accuracy').exists()).toBe(true)
    expect(w.find('#r-readability').exists()).toBe(true)
    expect(w.find('#r-value').exists()).toBe(true)
    expect(w.find('#review-title').exists()).toBe(true)
    expect(w.find('#review-content').exists()).toBe(true)
    expect(w.find('button[type="submit"]').exists()).toBe(true)
  })

  it('submit button is disabled when poem ID is missing', async () => {
    const { default: Page } = await import('./ReviewSubmitPage.vue')
    const w = mount(Page)
    await w.find('#poem-id').setValue('')
    const btn = w.find('button[type="submit"]')
    expect(btn.attributes('disabled')).toBeDefined()
  })

  it('calls submitReview with correct payload on submit', async () => {
    mockSubmitReview.mockResolvedValueOnce({ queued: false, data: { id: 1 } })
    const { default: Page } = await import('./ReviewSubmitPage.vue')
    const w = mount(Page)
    await w.find('#poem-id').setValue('5')
    await w.find('#r-accuracy').setValue('4')
    await w.find('#r-readability').setValue('5')
    await w.find('#r-value').setValue('3')
    await w.find('#review-title').setValue('Nice')
    await w.find('#review-content').setValue('A good poem')
    await w.find('form').trigger('submit')
    expect(mockSubmitReview).toHaveBeenCalledWith({
      poemId: 5,
      ratingAccuracy: 4,
      ratingReadability: 5,
      ratingValue: 3,
      title: 'Nice',
      content: 'A good poem',
    })
  })

  it('shows success message after successful submit', async () => {
    mockSubmitReview.mockResolvedValueOnce({ queued: false, data: { id: 2 } })
    const { default: Page } = await import('./ReviewSubmitPage.vue')
    const w = mount(Page)
    await w.find('#poem-id').setValue('1')
    await w.find('form').trigger('submit')
    await mockSubmitReview.mock.results[0].value
    reviewsState.submitResult.value = { queued: false, data: { id: 2 } }
    await w.vm.$nextTick()
    expect(w.text()).toContain('submitted successfully')
  })

  it('shows queued message when result has queued=true', async () => {
    const { default: Page } = await import('./ReviewSubmitPage.vue')
    const w = mount(Page)
    reviewsState.submitResult.value = { queued: true, entry: {} }
    await w.vm.$nextTick()
    expect(w.text()).toContain('offline')
  })

  it('shows error message on submit failure', async () => {
    const { default: Page } = await import('./ReviewSubmitPage.vue')
    const w = mount(Page)
    reviewsState.submitError.value = 'poem not found'
    await w.vm.$nextTick()
    expect(w.text()).toContain('poem not found')
  })
})
