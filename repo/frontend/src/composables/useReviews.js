import { ref } from 'vue'
import { apiWrite } from '../offline/api.js'

const submitting = ref(false)
const submitError = ref('')
const submitResult = ref(null)

export function useReviews() {
  async function submitReview({ poemId, ratingAccuracy, ratingReadability, ratingValue, title, content }) {
    submitting.value = true
    submitError.value = ''
    submitResult.value = null
    try {
      const res = await apiWrite({
        method: 'POST',
        url: '/api/v1/reviews',
        body: {
          poem_id: poemId,
          rating_accuracy: ratingAccuracy,
          rating_readability: ratingReadability,
          rating_value: ratingValue,
          title: title || '',
          content: content || '',
        },
        kind: 'review',
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

  return { submitting, submitError, submitResult, submitReview, reset }
}
