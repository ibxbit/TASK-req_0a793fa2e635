import { ref } from 'vue'
import { apiWrite } from '../offline/api.js'

const submitting = ref(false)
const submitError = ref('')
const submitResult = ref(null)

export function useComplaints() {
  async function submitComplaint({ subject, targetType, notes }) {
    submitting.value = true
    submitError.value = ''
    submitResult.value = null
    try {
      const res = await apiWrite({
        method: 'POST',
        url: '/api/v1/complaints',
        body: { subject, target_type: targetType, notes: notes || '' },
        kind: 'complaint',
      })
      submitResult.value = res
      return res
    } catch (err) {
      submitError.value = err?.response?.data?.error || err?.message || 'submission failed'
      throw err
    } finally {
      submitting.value = false
    }
  }

  function reset() {
    submitting.value = false
    submitError.value = ''
    submitResult.value = null
  }

  return { submitting, submitError, submitResult, submitComplaint, reset }
}
