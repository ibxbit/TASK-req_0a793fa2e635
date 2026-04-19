import { describe, it, expect, vi, beforeEach } from 'vitest'

const { axiosRequest } = vi.hoisted(() => ({
  axiosRequest: vi.fn(),
}))

vi.mock('axios', () => ({
  default: {
    create: () => ({
      request: (cfg) => axiosRequest(cfg),
      get: vi.fn(),
    }),
  },
}))

vi.mock('../offline/network.js', () => ({
  isOnline: { value: true },
  onNetworkChange: vi.fn(),
}))

beforeEach(() => {
  axiosRequest.mockReset()
})

describe('useReviews', () => {
  it('submitReview sends POST /api/v1/reviews with correct fields', async () => {
    axiosRequest.mockResolvedValueOnce({ data: { id: 42, status: 'pending' } })
    const { useReviews } = await import('./useReviews.js')
    const { submitReview, submitResult, submitError } = useReviews()

    const res = await submitReview({
      poemId: 1,
      ratingAccuracy: 4,
      ratingReadability: 5,
      ratingValue: 4,
      title: 'Great poem',
      content: 'Very nice',
    })

    expect(res.queued).toBe(false)
    expect(res.data).toEqual({ id: 42, status: 'pending' })
    expect(submitResult.value).toEqual(res)
    expect(submitError.value).toBe('')

    const cfg = axiosRequest.mock.calls[0][0]
    expect(cfg.method).toBe('POST')
    expect(cfg.url).toBe('/api/v1/reviews')
    expect(cfg.data.poem_id).toBe(1)
    expect(cfg.data.rating_accuracy).toBe(4)
    expect(cfg.data.rating_readability).toBe(5)
    expect(cfg.data.rating_value).toBe(4)
    expect(cfg.headers['Idempotency-Key']).toBeTruthy()
  })

  it('queues review when offline', async () => {
    const net = await import('../offline/network.js')
    net.isOnline.value = false
    const { useReviews } = await import('./useReviews.js')
    const { submitReview, submitResult } = useReviews()

    const res = await submitReview({
      poemId: 2, ratingAccuracy: 3, ratingReadability: 3, ratingValue: 3,
    })

    expect(res.queued).toBe(true)
    expect(submitResult.value.queued).toBe(true)
    expect(axiosRequest).not.toHaveBeenCalled()
    net.isOnline.value = true
  })

  it('queues review on network error', async () => {
    axiosRequest.mockRejectedValueOnce(Object.assign(new Error('net'), { response: undefined }))
    const { useReviews } = await import('./useReviews.js')
    const { submitReview, submitResult } = useReviews()

    const res = await submitReview({
      poemId: 3, ratingAccuracy: 2, ratingReadability: 2, ratingValue: 2,
    })

    expect(res.queued).toBe(true)
    expect(submitResult.value.queued).toBe(true)
  })

  it('sets submitError and rethrows on 4xx', async () => {
    axiosRequest.mockRejectedValueOnce({ response: { status: 400, data: { error: 'invalid rating' } } })
    const { useReviews } = await import('./useReviews.js')
    const { submitReview, submitError } = useReviews()

    await expect(submitReview({
      poemId: 4, ratingAccuracy: 9, ratingReadability: 9, ratingValue: 9,
    })).rejects.toBeDefined()

    expect(submitError.value).toBe('invalid rating')
  })

  it('reset clears state', async () => {
    axiosRequest.mockResolvedValueOnce({ data: { id: 1 } })
    const { useReviews } = await import('./useReviews.js')
    const { submitReview, submitResult, submitError, reset } = useReviews()

    await submitReview({ poemId: 1, ratingAccuracy: 3, ratingReadability: 3, ratingValue: 3 })
    expect(submitResult.value).not.toBeNull()
    reset()
    expect(submitResult.value).toBeNull()
    expect(submitError.value).toBe('')
  })
})
