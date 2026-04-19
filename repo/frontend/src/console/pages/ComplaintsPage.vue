<script setup>
import { ref, onMounted } from 'vue'
import { get, post } from '../api.js'

const items = ref([])
const statuses = ref([])
const selected = ref(null)
const error = ref('')

async function load() {
  try { items.value = (await get('/complaints')).items || [] }
  catch (e) { error.value = e?.response?.data?.error || e.message }
}

async function loadStatuses() {
  try { statuses.value = (await get('/arbitration/statuses')).items || [] } catch {}
}

async function open(id) {
  try { selected.value = await get(`/complaints/${id}`) }
  catch (e) { error.value = e?.response?.data?.error || e.message }
}

async function assign(id) {
  const uid = prompt('Arbitrator user ID:')
  if (!uid) return
  await post(`/complaints/${id}/assign`, { arbitrator_id: Number(uid) })
  await load(); await open(id)
}

async function resolve(id) {
  const code = prompt(`Arbitration code (${statuses.value.map(s => s.code).join(', ')}):`)
  if (!code) return
  const resolution = prompt('Resolution text:') || ''
  await post(`/complaints/${id}/resolve`, { arbitration_code: code, resolution })
  await load(); await open(id)
}

onMounted(() => { load(); loadStatuses() })
</script>

<template>
  <div class="cx">
    <aside class="list">
      <h3>Complaints</h3>
      <ul>
        <li
          v-for="c in items"
          :key="c.id"
          :class="{ sel: selected?.id === c.id }"
          @click="open(c.id)"
        >
          <strong>#{{ c.id }}</strong> · {{ c.subject }}
          <div class="meta">{{ c.target_type }} · {{ c.arbitration_code || 'pending' }}</div>
        </li>
        <li v-if="!items.length" class="empty">None</li>
      </ul>
    </aside>
    <section class="detail" v-if="selected">
      <h3>#{{ selected.id }} — {{ selected.subject }}</h3>
      <p><strong>Target:</strong> {{ selected.target_type }} {{ selected.target_id ?? '' }}</p>
      <p><strong>Status:</strong> {{ selected.arbitration_code }}</p>
      <p><strong>Arbitrator:</strong> {{ selected.arbitrator_id ?? 'unassigned' }}</p>
      <pre class="notes">{{ selected.notes || '(no notes)' }}</pre>
      <p v-if="selected.resolution"><strong>Resolution:</strong> {{ selected.resolution }}</p>
      <div class="actions">
        <button @click="assign(selected.id)">Assign arbitrator</button>
        <button @click="resolve(selected.id)">Set status / resolve</button>
      </div>
    </section>
    <p v-if="error" class="err">{{ error }}</p>
  </div>
</template>

<style scoped>
.cx { display: grid; grid-template-columns: 22em 1fr; gap: 1em; align-items: start; }
.list, .detail { background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 1em; }
.list h3, .detail h3 { margin: 0 0 0.5em; }
ul { list-style: none; margin: 0; padding: 0; }
li { padding: 0.5em; border-bottom: 1px solid #f3f4f6; cursor: pointer; border-radius: 4px; }
li:hover { background: #f9fafb; }
li.sel { background: #eff6ff; }
.meta { font-size: 0.8em; color: #6b7280; }
.empty { color: #9ca3af; cursor: default; }
.notes {
  background: #f9fafb; padding: 0.75em; border-radius: 4px;
  white-space: pre-wrap; font-family: ui-monospace, monospace; font-size: 0.9em;
  max-height: 18em; overflow: auto;
}
.actions { margin-top: 0.75em; display: flex; gap: 0.5em; }
.actions button { padding: 0.4em 0.9em; border: 1px solid #2563eb; background: #2563eb; color: white; border-radius: 4px; cursor: pointer; }
.err { color: #b91c1c; grid-column: 1/-1; }
</style>
