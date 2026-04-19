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

describe('useComplaints', () => {
  it('submitComplaint sends POST /api/v1/complaints with correct fields', async () => {
    axiosRequest.mockResolvedValueOnce({ data: { id: 7, arbitration_code: 'submitted' } })
    const { useComplaints } = await import('./useComplaints.js')
    const { submitComplaint, submitResult, submitError } = useComplaints()

    const res = await submitComplaint({
      subject: 'Spam content',
      targetType: 'poem',
      notes: 'See attached',
    })

    expect(res.queued).toBe(false)
    expect(res.data).toEqual({ id: 7, arbitration_code: 'submitted' })
    expect(submitResult.value).toEqual(res)
    expect(submitError.value).toBe('')

    const cfg = axiosRequest.mock.calls[0][0]
    expect(cfg.method).toBe('POST')
    expect(cfg.url).toBe('/api/v1/complaints')
    expect(cfg.data.subject).toBe('Spam content')
    expect(cfg.data.target_type).toBe('poem')
    expect(cfg.data.notes).toBe('See attached')
    expect(cfg.headers['Idempotency-Key']).toBeTruthy()
  })

  it('queues complaint when offline', async () => {
    const net = await import('../offline/network.js')
    net.isOnline.value = false
    const { useComplaints } = await import('./useComplaints.js')
    const { submitComplaint, submitResult } = useComplaints()

    const res = await submitComplaint({ subject: 'Offline test', targetType: 'other', notes: '' })

    expect(res.queued).toBe(true)
    expect(submitResult.value.queued).toBe(true)
    expect(axiosRequest).not.toHaveBeenCalled()
    net.isOnline.value = true
  })

  it('queues complaint on network error', async () => {
    axiosRequest.mockRejectedValueOnce(Object.assign(new Error('net'), { response: undefined }))
    const { useComplaints } = await import('./useComplaints.js')
    const { submitComplaint, submitResult } = useComplaints()

    const res = await submitComplaint({ subject: 'Net fail', targetType: 'review', notes: '' })

    expect(res.queued).toBe(true)
    expect(submitResult.value.queued).toBe(true)
  })

  it('sets submitError and rethrows on 4xx', async () => {
    axiosRequest.mockRejectedValueOnce({ response: { status: 400, data: { error: 'invalid target_type' } } })
    const { useComplaints } = await import('./useComplaints.js')
    const { submitComplaint, submitError } = useComplaints()

    await expect(submitComplaint({ subject: 'Bad', targetType: 'planet', notes: '' })).rejects.toBeDefined()

    expect(submitError.value).toBe('invalid target_type')
  })

  it('reset clears state', async () => {
    axiosRequest.mockResolvedValueOnce({ data: { id: 5 } })
    const { useComplaints } = await import('./useComplaints.js')
    const { submitComplaint, submitResult, submitError, reset } = useComplaints()

    await submitComplaint({ subject: 'x', targetType: 'other', notes: '' })
    expect(submitResult.value).not.toBeNull()
    reset()
    expect(submitResult.value).toBeNull()
    expect(submitError.value).toBe('')
  })
})
