import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { resumableDownload, clearDownload } = vi.hoisted(() => ({
  resumableDownload: vi.fn(),
  clearDownload: vi.fn(),
}))

vi.mock('../../offline/download.js', () => ({
  resumableDownload,
  clearDownload,
}))

vi.mock('../../offline/network.js', async () => {
  const { ref } = await import('vue')
  return {
    isOnline: ref(true),
    onNetworkChange: vi.fn(),
  }
})

beforeEach(() => {
  resumableDownload.mockReset()
  clearDownload.mockReset()
})

describe('ContentPackPage.vue', () => {
  it('starts a download and reports progress', async () => {
    resumableDownload.mockImplementationOnce(async ({ onProgress }) => {
      onProgress({ downloaded: 512, total: 1024 })
      onProgress({ downloaded: 1024, total: 1024 })
      return new Uint8Array(1024)
    })
    const { default: Page } = await import('./ContentPackPage.vue')
    const w = mount(Page)
    await w.find('[data-test="dl-start"]').trigger('click')
    await flushPromises()
    expect(resumableDownload).toHaveBeenCalledTimes(1)
    const [args] = resumableDownload.mock.calls[0]
    expect(args.url).toBe('/api/v1/content-packs/current')
    expect(args.key).toBe('content-pack:current')
    expect(w.text()).toContain('100%')
    expect(w.text()).toContain('state: complete')
  })

  it('pause aborts the in-flight download and marks state paused', async () => {
    let observedSignal = null
    // Resolve after start() so we can click Pause while it's "in flight".
    resumableDownload.mockImplementationOnce(async ({ signal }) => {
      observedSignal = signal
      // Block until the signal fires.
      await new Promise((_, reject) => {
        signal.addEventListener('abort', () => {
          const e = new Error('aborted')
          e.name = 'AbortError'
          reject(e)
        })
      })
    })
    const { default: Page } = await import('./ContentPackPage.vue')
    const w = mount(Page)
    // Kick off but don't await — otherwise we'd block on the promise.
    w.find('[data-test="dl-start"]').trigger('click')
    await flushPromises()
    // Pause button appears while running.
    expect(w.find('[data-test="dl-pause"]').exists()).toBe(true)
    await w.find('[data-test="dl-pause"]').trigger('click')
    await flushPromises()
    expect(observedSignal).not.toBeNull()
    expect(observedSignal.aborted).toBe(true)
    expect(w.text()).toContain('state: paused')
  })

  it('Reset clears stored state and restores the idle UI', async () => {
    const { default: Page } = await import('./ContentPackPage.vue')
    const w = mount(Page)
    await w.find('[data-test="dl-reset"]').trigger('click')
    await flushPromises()
    expect(clearDownload).toHaveBeenCalledWith('content-pack:current')
    expect(w.find('[data-test="dl-start"]').text()).toBe('Start')
    expect(w.find('[data-test="dl-stats"]').exists()).toBe(false)
  })

  it('shows the error from a failed download', async () => {
    resumableDownload.mockRejectedValueOnce(new Error('HEAD failed: 503'))
    const { default: Page } = await import('./ContentPackPage.vue')
    const w = mount(Page)
    await w.find('[data-test="dl-start"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-test="dl-error"]').text()).toContain('HEAD failed')
    expect(w.text()).toContain('state: error')
  })

  it('blocks start when offline', async () => {
    const net = await import('../../offline/network.js')
    net.isOnline.value = false
    const { default: Page } = await import('./ContentPackPage.vue')
    const w = mount(Page)
    // Button is disabled; verify the underlying attribute is present.
    expect(w.find('[data-test="dl-start"]').attributes('disabled')).toBeDefined()
    net.isOnline.value = true
  })
})
