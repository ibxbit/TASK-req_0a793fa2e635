<script setup>
import { ref, computed } from 'vue'
import { useComplaints } from '../composables/useComplaints.js'

const { submitting, submitError, submitResult, submitComplaint, reset } = useComplaints()

const TARGET_TYPES = ['poem', 'review', 'user', 'author', 'other']

const subject = ref('')
const targetType = ref('other')
const notes = ref('')

const queued = computed(() => submitResult.value?.queued === true)
const submitted = computed(() => submitResult.value != null && !queued.value)

const validationError = computed(() => {
  if (!subject.value.trim()) return 'Subject is required'
  if (!TARGET_TYPES.includes(targetType.value)) return 'Invalid target type'
  return ''
})

async function onSubmit() {
  if (validationError.value) return
  reset()
  try {
    await submitComplaint({
      subject: subject.value.trim(),
      targetType: targetType.value,
      notes: notes.value,
    })
  } catch {
    // submitError is set by composable
  }
}

function startNew() {
  reset()
  subject.value = ''
  targetType.value = 'other'
  notes.value = ''
}
</script>

<template>
  <div class="complaint-page">
    <h2>Submit a Complaint</h2>

    <div v-if="submitted" class="success" role="status">
      Complaint submitted successfully.
      <button @click="startNew">Submit another</button>
    </div>

    <div v-else-if="queued" class="queued" role="status">
      You appear to be offline. Your complaint has been saved and will be submitted automatically when connectivity is restored.
      <button @click="startNew">Submit another</button>
    </div>

    <form v-else @submit.prevent="onSubmit" novalidate>
      <div class="field">
        <label for="subject">Subject</label>
        <input id="subject" v-model="subject" type="text" maxlength="255" placeholder="Brief subject of your complaint" required />
      </div>

      <div class="field">
        <label for="target-type">Category</label>
        <select id="target-type" v-model="targetType">
          <option v-for="t in TARGET_TYPES" :key="t" :value="t">{{ t }}</option>
        </select>
      </div>

      <div class="field">
        <label for="notes">Details <span class="opt">(optional)</span></label>
        <textarea id="notes" v-model="notes" rows="5" maxlength="4000" placeholder="Provide additional context…"></textarea>
      </div>

      <p v-if="validationError" class="error" role="alert">{{ validationError }}</p>
      <p v-if="submitError" class="error" role="alert">{{ submitError }}</p>

      <button type="submit" :disabled="submitting || !!validationError">
        {{ submitting ? 'Submitting…' : 'Submit Complaint' }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.complaint-page { max-width: 36em; padding: 1.5em 0; }
h2 { margin-top: 0; }
.field { display: flex; flex-direction: column; gap: 0.3em; margin-bottom: 1em; }
label { font-size: 0.9em; font-weight: 500; }
.opt { font-weight: 400; color: #6b7280; font-size: 0.85em; }
input[type="text"], select, textarea {
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
