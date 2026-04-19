<script setup>
import { ref, computed } from 'vue'
import { useReviews } from '../composables/useReviews.js'

const { submitting, submitError, submitResult, submitReview, reset } = useReviews()

const poemId = ref('')
const ratingAccuracy = ref(3)
const ratingReadability = ref(3)
const ratingValue = ref(3)
const title = ref('')
const content = ref('')

const queued = computed(() => submitResult.value?.queued === true)
const submitted = computed(() => submitResult.value != null && !queued.value)

const validationError = computed(() => {
  if (!poemId.value || isNaN(Number(poemId.value)) || Number(poemId.value) <= 0) return 'Poem ID is required'
  for (const r of [ratingAccuracy.value, ratingReadability.value, ratingValue.value]) {
    if (r < 1 || r > 5) return 'Ratings must be between 1 and 5'
  }
  return ''
})

async function onSubmit() {
  if (validationError.value) return
  reset()
  try {
    await submitReview({
      poemId: Number(poemId.value),
      ratingAccuracy: Number(ratingAccuracy.value),
      ratingReadability: Number(ratingReadability.value),
      ratingValue: Number(ratingValue.value),
      title: title.value,
      content: content.value,
    })
  } catch {
    // submitError is set by composable
  }
}

function startNew() {
  reset()
  poemId.value = ''
  ratingAccuracy.value = 3
  ratingReadability.value = 3
  ratingValue.value = 3
  title.value = ''
  content.value = ''
}
</script>

<template>
  <div class="review-page">
    <h2>Submit a Review</h2>

    <div v-if="submitted" class="success" role="status">
      Review submitted successfully.
      <button @click="startNew">Submit another</button>
    </div>

    <div v-else-if="queued" class="queued" role="status">
      You appear to be offline. Your review has been saved and will be submitted automatically when connectivity is restored.
      <button @click="startNew">Submit another</button>
    </div>

    <form v-else @submit.prevent="onSubmit" novalidate>
      <div class="field">
        <label for="poem-id">Poem ID</label>
        <input id="poem-id" v-model="poemId" type="number" min="1" placeholder="Enter the poem ID" required />
      </div>

      <fieldset>
        <legend>Ratings (1–5)</legend>
        <div class="rating-row">
          <label for="r-accuracy">Accuracy</label>
          <input id="r-accuracy" v-model.number="ratingAccuracy" type="number" min="1" max="5" />
        </div>
        <div class="rating-row">
          <label for="r-readability">Readability</label>
          <input id="r-readability" v-model.number="ratingReadability" type="number" min="1" max="5" />
        </div>
        <div class="rating-row">
          <label for="r-value">Value</label>
          <input id="r-value" v-model.number="ratingValue" type="number" min="1" max="5" />
        </div>
      </fieldset>

      <div class="field">
        <label for="review-title">Title <span class="opt">(optional)</span></label>
        <input id="review-title" v-model="title" type="text" maxlength="255" />
      </div>

      <div class="field">
        <label for="review-content">Review <span class="opt">(optional)</span></label>
        <textarea id="review-content" v-model="content" rows="5" maxlength="4000"></textarea>
      </div>

      <p v-if="validationError" class="error" role="alert">{{ validationError }}</p>
      <p v-if="submitError" class="error" role="alert">{{ submitError }}</p>

      <button type="submit" :disabled="submitting || !!validationError">
        {{ submitting ? 'Submitting…' : 'Submit Review' }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.review-page { max-width: 36em; padding: 1.5em 0; }
h2 { margin-top: 0; }
.field { display: flex; flex-direction: column; gap: 0.3em; margin-bottom: 1em; }
fieldset { border: 1px solid #e5e7eb; border-radius: 6px; margin-bottom: 1em; padding: 0.75em 1em; }
legend { font-weight: 600; font-size: 0.9em; }
.rating-row { display: flex; align-items: center; gap: 0.75em; margin-bottom: 0.4em; }
.rating-row label { width: 7em; }
.rating-row input { width: 4em; }
label { font-size: 0.9em; font-weight: 500; }
.opt { font-weight: 400; color: #6b7280; font-size: 0.85em; }
input[type="text"], input[type="number"], textarea {
  border: 1px solid #d1d5db; border-radius: 4px; padding: 0.4em 0.6em;
  font-size: 0.95em; font-family: inherit; width: 100%;
}
textarea { resize: vertical; }
button[type="submit"] {
  background: #1f2937; color: #fff; border: none; border-radius: 4px;
  padding: 0.5em 1.25em; font-size: 0.95em; cursor: pointer;
}
button[type="submit"]:disabled { opacity: 0.6; cursor: not-allowed; }
.error { color: #dc2626; font-size: 0.9em; margin: 0.5em 0; }
.success { background: #d1fae5; border: 1px solid #6ee7b7; border-radius: 6px; padding: 1em; }
.queued { background: #fef3c7; border: 1px solid #fcd34d; border-radius: 6px; padding: 1em; }
.success button, .queued button { margin-top: 0.75em; }
</style>
