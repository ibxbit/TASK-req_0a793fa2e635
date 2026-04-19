<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { get, post } from '../api.js'

const items = ref([])
const error = ref('')

async function load() {
  try { items.value = (await get('/approvals')).items || [] }
  catch (e) { error.value = e?.response?.data?.error || e.message }
}

async function decide(batchID, verb) {
  if (verb === 'reject' && !confirm('Reject & revert this batch?')) return
  try {
    await post(`/approvals/${batchID}/${verb}`, {})
    await load()
  } catch (e) { error.value = e?.response?.data?.error || e.message }
}

let timer
onMounted(() => { load(); timer = setInterval(load, 10_000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="!items.length" class="empty">No pending approval batches.</p>

    <article v-for="b in items" :key="b.batch_id" class="card">
      <header>
        <code class="bid">{{ b.batch_id.slice(0, 16) }}…</code>
        <span>{{ b.entries }} entries</span>
        <span>deadline: {{ new Date(b.deadline).toLocaleString() }}</span>
      </header>
      <p class="actions">
        <strong>Actions:</strong>
        <span v-for="a in b.actions" :key="a" class="tag">{{ a }}</span>
      </p>
      <div class="btns">
        <button class="ok"  @click="decide(b.batch_id, 'approve')">Approve</button>
        <button class="bad" @click="decide(b.batch_id, 'reject')">Reject & Revert</button>
      </div>
    </article>
  </div>
</template>

<style scoped>
.empty { color: #6b7280; }
.err { color: #b91c1c; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 1em; margin-bottom: 0.75em; }
.card header { display: flex; gap: 1em; align-items: center; font-size: 0.85em; color: #6b7280; margin-bottom: 0.5em; }
.bid { font-family: ui-monospace, monospace; }
.tag { display: inline-block; background: #f3f4f6; padding: 0.1em 0.5em; border-radius: 999px; font-size: 0.8em; margin-right: 0.3em; }
.btns { display: flex; gap: 0.5em; }
.ok  { padding: 0.4em 1em; background: #059669; color: white; border: none; border-radius: 4px; cursor: pointer; }
.bad { padding: 0.4em 1em; background: #b91c1c; color: white; border: none; border-radius: 4px; cursor: pointer; }
</style>
