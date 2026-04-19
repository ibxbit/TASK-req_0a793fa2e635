<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { get } from '../api.js'

const filters = reactive({
  entity_type: '',
  action: '',
  approval_status: '',
  actor_id: '',
})
const items = ref([])
const error = ref('')
const expanded = ref(null)

async function load() {
  const params = {}
  for (const [k, v] of Object.entries(filters)) {
    if (v !== '' && v !== null) params[k] = v
  }
  params.limit = 100
  try { items.value = (await get('/audit-logs', params)).items || [] }
  catch (e) { error.value = e?.response?.data?.error || e.message }
}

onMounted(load)
watch(filters, load)
</script>

<template>
  <div>
    <form class="filters" @submit.prevent="load">
      <input placeholder="entity_type (poem, author, …)" v-model="filters.entity_type" />
      <select v-model="filters.action">
        <option value="">action: any</option>
        <option>create</option><option>update</option><option>delete</option><option>restore</option>
      </select>
      <select v-model="filters.approval_status">
        <option value="">approval: any</option>
        <option>not_required</option><option>pending</option>
        <option>approved</option><option>rejected</option><option>reverted</option>
      </select>
      <input placeholder="actor_id" v-model="filters.actor_id" />
    </form>

    <p v-if="error" class="err">{{ error }}</p>

    <table>
      <thead>
        <tr>
          <th>ID</th><th>When</th><th>Actor</th><th>Action</th>
          <th>Entity</th><th>Approval</th><th></th>
        </tr>
      </thead>
      <tbody>
        <template v-for="e in items" :key="e.id">
          <tr :class="{ sel: expanded === e.id }">
            <td>{{ e.id }}</td>
            <td>{{ new Date(e.created_at).toLocaleString() }}</td>
            <td>{{ e.actor_role || '—' }} #{{ e.actor_id ?? '—' }}</td>
            <td>{{ e.action }}</td>
            <td>{{ e.entity_type }} #{{ e.entity_id ?? '—' }}</td>
            <td :class="e.approval_status">{{ e.approval_status }}</td>
            <td><button @click="expanded = expanded === e.id ? null : e.id">{{ expanded === e.id ? '−' : '▸' }}</button></td>
          </tr>
          <tr v-if="expanded === e.id" class="detail">
            <td colspan="7">
              <div class="diff">
                <section><h5>Before</h5><pre>{{ e.before ? JSON.stringify(e.before, null, 2) : '(null)' }}</pre></section>
                <section><h5>After</h5><pre>{{ e.after ? JSON.stringify(e.after, null, 2) : '(null)' }}</pre></section>
              </div>
              <p v-if="e.batch_id" class="meta">batch: <code>{{ e.batch_id }}</code></p>
            </td>
          </tr>
        </template>
        <tr v-if="!items.length"><td colspan="7" class="empty">No entries.</td></tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.filters { display: flex; gap: 0.5em; margin-bottom: 1em; flex-wrap: wrap; }
.filters input, .filters select { padding: 0.35em 0.5em; border: 1px solid #ccc; border-radius: 4px; font-size: 0.9em; }
.err { color: #b91c1c; }
table { width: 100%; border-collapse: collapse; font-size: 0.9em; background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; }
th, td { padding: 0.4em 0.55em; border-bottom: 1px solid #f3f4f6; text-align: left; vertical-align: top; }
th { color: #6b7280; }
tr.sel { background: #eff6ff; }
.detail td { background: #f9fafb; }
.diff { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75em; }
.diff pre { margin: 0; font-size: 0.8em; background: #fff; padding: 0.5em; border: 1px solid #e5e7eb; border-radius: 4px; max-height: 18em; overflow: auto; }
h5 { margin: 0 0 0.25em; font-size: 0.8em; color: #6b7280; text-transform: uppercase; }
.meta { font-size: 0.8em; color: #6b7280; margin: 0.5em 0 0; }
code { font-family: ui-monospace, monospace; }
.pending  { color: #ca8a04; }
.approved { color: #059669; }
.rejected,.reverted { color: #b91c1c; }
.empty { color: #6b7280; text-align: center; }
</style>
