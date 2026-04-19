import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

// Only vi.fn() instances inside vi.hoisted() — imported symbols (ref) are not
// yet available when the hoisted factory runs, so refs live at module scope.
const { mockSubmitComplaint, mockReset } = vi.hoisted(() => ({
  mockSubmitComplaint: vi.fn(),
  mockReset: vi.fn(),
}))

// Reactive state at module scope: imports are resolved before this runs.
// The vi.mock() factory below closes over complaintsState and is called lazily
// (when the component first imports useComplaints), so complaintsState is ready.
const complaintsState = {
  submitting: ref(false),
  submitError: ref(''),
  submitResult: ref(null),
  submitComplaint: mockSubmitComplaint,
  reset: mockReset,
}

vi.mock('../composables/useComplaints.js', () => ({
  useComplaints: () => complaintsState,
}))

beforeEach(() => {
  complaintsState.submitting.value = false
  complaintsState.submitError.value = ''
  complaintsState.submitResult.value = null
  mockSubmitComplaint.mockReset()
  mockReset.mockReset()
})

describe('ComplaintSubmitPage.vue', () => {
  it('renders the form fields', async () => {
    const { default: Page } = await import('./ComplaintSubmitPage.vue')
    const w = mount(Page)
    expect(w.find('#subject').exists()).toBe(true)
    expect(w.find('#target-type').exists()).toBe(true)
    expect(w.find('#notes').exists()).toBe(true)
    expect(w.find('button[type="submit"]').exists()).toBe(true)
  })

  it('submit button is disabled when subject is empty', async () => {
    const { default: Page } = await import('./ComplaintSubmitPage.vue')
    const w = mount(Page)
    await w.find('#subject').setValue('')
    const btn = w.find('button[type="submit"]')
    expect(btn.attributes('disabled')).toBeDefined()
  })

  it('calls submitComplaint with correct payload', async () => {
    mockSubmitComplaint.mockResolvedValueOnce({ queued: false, data: { id: 3 } })
    const { default: Page } = await import('./ComplaintSubmitPage.vue')
    const w = mount(Page)
    await w.find('#subject').setValue('Offensive content')
    await w.find('#target-type').setValue('poem')
    await w.find('#notes').setValue('See page 3')
    await w.find('form').trigger('submit')
    expect(mockSubmitComplaint).toHaveBeenCalledWith({
      subject: 'Offensive content',
      targetType: 'poem',
      notes: 'See page 3',
    })
  })

  it('shows success message after successful submit', async () => {
    const { default: Page } = await import('./ComplaintSubmitPage.vue')
    const w = mount(Page)
    complaintsState.submitResult.value = { queued: false, data: { id: 5 } }
    await w.vm.$nextTick()
    expect(w.text()).toContain('submitted successfully')
  })

  it('shows queued message when offline', async () => {
    const { default: Page } = await import('./ComplaintSubmitPage.vue')
    const w = mount(Page)
    complaintsState.submitResult.value = { queued: true, entry: {} }
    await w.vm.$nextTick()
    expect(w.text()).toContain('offline')
  })

  it('shows error message on submit failure', async () => {
    const { default: Page } = await import('./ComplaintSubmitPage.vue')
    const w = mount(Page)
    complaintsState.submitError.value = 'invalid target_type'
    await w.vm.$nextTick()
    expect(w.text()).toContain('invalid target_type')
  })

  it('target type dropdown contains expected options', async () => {
    const { default: Page } = await import('./ComplaintSubmitPage.vue')
    const w = mount(Page)
    const options = w.findAll('#target-type option').map(o => o.element.value)
    expect(options).toContain('poem')
    expect(options).toContain('review')
    expect(options).toContain('user')
    expect(options).toContain('other')
  })
})
