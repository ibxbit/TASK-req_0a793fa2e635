<script setup>
// Revision history console — admin only (route guard enforces it).
//
// The user types an entity type and id; we hit GET /revisions and render
// a chronological history. Each restorable row exposes a "Restore" button
// that POSTs /revisions/:id/restore, then reloads the list so the new
// action=restore audit row shows up.
import { ref, onMounted } from 'vue'
import { get, post } from '../api.js'

const entityType = ref('dynasty')
const entityId   = ref('')
const items      = ref([])
const supported  = ref([])
const retention  = ref(30)
const error      = ref('')
const busy       = ref(false)
const justRestored = ref(null)

onMounted(async () => {
  try {
    const d = await get('/revisions/supported-entities')
    supported.value = d.items || []
    retention.value = d.retention_days || 30
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
  }
})

async function load() {
  error.value = ''
  items.value = []
  if (!entityType.value || !entityId.value) {
    error.value = 'entity_type and entity_id are required'
    return
  }
  try {
    const d = await get('/revisions', {
      entity_type: entityType.value,
      entity_id: Number(entityId.value),
      limit: 100,
    })
    items.value = d.items || []
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
  }
}

async function onRestore(rev) {
  const summary = `restore ${rev.entity_type} #${rev.entity_id} back from ${rev.action}`
  if (!confirm(`Will ${summary}. Proceed?`)) return
  busy.value = true
  error.value = ''
  try {
    const r = await post(`/revisions/${rev.id}/restore`)
    justRestored.value = r
    await load()
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
  } finally {
    busy.value = false
  }
}

function shortJson(raw) {
  if (!raw) return ''
  try {
    return JSON.stringify(raw).slice(0, 120)
  } catch {
    return String(raw).slice(0, 120)
  }
}
</script>

<template>
  <div class="revisions">
    <div class="card lookup">
      <h3>Lookup revision history</h3>
      <p class="hint">
        Revisions older than <strong>{{ retention }} days</strong> are no longer
        restorable.
      </p>
      <form @submit.prevent="load">
        <select v-model="entityType" required>
          <option v-for="s in supported" :key="s" :value="s">{{ s }}</option>
        </select>
        <input v-model="entityId" type="number" min="1" placeholder="entity id" required />
        <button type="submit" :disabled="busy">{{ busy ? '…' : 'Load' }}</button>
      </form>
    </div>

    <p v-if="error" class="err" data-test="rev-error">{{ error }}</p>

    <p v-if="justRestored" class="ok" data-test="rev-ok">
      Restored revision #{{ justRestored.restored_revision_id }}
      ({{ justRestored.action_restored }} of {{ justRestored.entity_type }}
      #{{ justRestored.entity_id }}).
    </p>

    <table v-if="items.length" class="list">
      <thead>
        <tr>
          <th>#</th>
          <th>action</th>
          <th>actor</th>
          <th>created</th>
          <th>before</th>
          <th>after</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in items" :key="r.id">
          <td class="id">{{ r.id }}</td>
          <td>{{ r.action }}</td>
          <td>{{ r.actor_role || '—' }}</td>
          <td class="ts">{{ r.created_at }}</td>
          <td class="json">{{ shortJson(r.before) }}</td>
          <td class="json">{{ shortJson(r.after) }}</td>
          <td>
            <button
              v-if="r.restorable"
              class="restore"
              @click="onRestore(r)"
              :disabled="busy"
            >Restore</button>
            <span v-else class="muted">—</span>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-else-if="!error" class="empty">Nothing loaded yet.</p>
  </div>
</template>

<style scoped>
.revisions { display: flex; flex-direction: column; gap: 1em; }
.card {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 1em;
}
.card h3 { margin: 0 0 0.35em; }
.hint { color: #6b7280; font-size: 0.85em; }
.lookup form { display: flex; gap: 0.4em; align-items: center; flex-wrap: wrap; margin-top: 0.5em; }
.lookup select, .lookup input {
  padding: 0.4em 0.6em;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 0.95em;
}
.lookup button {
  padding: 0.4em 1em;
  border-radius: 4px;
  background: #1f2937;
  color: #fff;
  border: none;
  cursor: pointer;
}
.err { color: #b91c1c; margin: 0; }
.ok  { color: #065f46; margin: 0; }
.list {
  width: 100%;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  border-collapse: collapse;
  font-size: 0.88em;
}
.list th, .list td {
  padding: 0.45em 0.75em;
  text-align: left;
  border-bottom: 1px solid #f3f4f6;
  vertical-align: top;
}
.list th { background: #f9fafb; }
.id { color: #9ca3af; width: 4em; }
.ts { color: #6b7280; white-space: nowrap; }
.json {
  font-family: ui-monospace, monospace;
  color: #111827;
  word-break: break-all;
  max-width: 18em;
}
.restore {
  padding: 0.2em 0.7em;
  border-radius: 4px;
  background: #d1fae5;
  color: #065f46;
  border: 1px solid #a7f3d0;
  cursor: pointer;
}
.muted { color: #9ca3af; }
.empty { color: #6b7280; font-size: 0.9em; }
</style>
