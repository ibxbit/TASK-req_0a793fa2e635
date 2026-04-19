<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { get, post } from '../api.js'

const jobs    = ref([])
const nodes   = ref([])
const metrics = ref(null)
const openId  = ref(null)
const error   = ref('')

const form = reactive({
  job_name: '',
  source_url: '',
  priority: 0,
  max_attempts: 5,
  daily_quota: 10000,
})

async function loadAll() {
  try {
    jobs.value  = (await get('/crawl/jobs')).items  || []
    nodes.value = (await get('/crawl/nodes')).items || []
  } catch (e) { error.value = e?.response?.data?.error || e.message }
}

async function openJob(id) {
  openId.value = id
  try { metrics.value = await get(`/crawl/jobs/${id}/metrics`) } catch {}
}

async function create() {
  error.value = ''
  try {
    await post('/crawl/jobs', {
      job_name: form.job_name,
      source_url: form.source_url,
      priority: Number(form.priority) || 0,
      max_attempts: Number(form.max_attempts) || 5,
      daily_quota: Number(form.daily_quota) || 10000,
    })
    form.job_name = ''; form.source_url = ''
    await loadAll()
  } catch (e) { error.value = e?.response?.data?.error || e.message }
}

async function action(id, verb) {
  try {
    await post(`/crawl/jobs/${id}/${verb}`, {})
    await loadAll()
    if (openId.value === id) await openJob(id)
  } catch (e) { error.value = e?.response?.data?.error || e.message }
}

let timer
onMounted(() => { loadAll(); timer = setInterval(loadAll, 5000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="crawl">
    <section class="panel">
      <h3>Nodes</h3>
      <table>
        <thead><tr><th>ID</th><th>Name</th><th>Host</th><th>Status</th><th>Heartbeat</th></tr></thead>
        <tbody>
          <tr v-for="n in nodes" :key="n.id">
            <td>{{ n.id }}</td>
            <td>{{ n.node_name }}</td>
            <td>{{ n.host }}</td>
            <td :class="n.status">{{ n.status }}</td>
            <td>{{ n.last_heartbeat_at ? new Date(n.last_heartbeat_at).toLocaleTimeString() : '—' }}</td>
          </tr>
          <tr v-if="!nodes.length"><td colspan="5" class="empty">No nodes online</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel">
      <h3>Create job</h3>
      <form @submit.prevent="create" class="form">
        <input placeholder="job_name" v-model="form.job_name" required />
        <input placeholder="source_url" v-model="form.source_url" />
        <input placeholder="priority" type="number" v-model.number="form.priority" />
        <input placeholder="max_attempts (≤5)" type="number" v-model.number="form.max_attempts" />
        <input placeholder="daily_quota" type="number" v-model.number="form.daily_quota" />
        <button type="submit">Queue</button>
      </form>
      <p v-if="error" class="err">{{ error }}</p>
    </section>

    <section class="panel">
      <h3>Jobs</h3>
      <table>
        <thead><tr><th>ID</th><th>Name</th><th>Status</th><th>Attempts</th><th>Pages</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="j in jobs" :key="j.id" :class="{ sel: openId === j.id }" @click="openJob(j.id)">
            <td>{{ j.id }}</td>
            <td>{{ j.job_name }}</td>
            <td :class="j.status">{{ j.status }}</td>
            <td>{{ j.attempts }}/{{ j.max_attempts }}</td>
            <td>{{ j.pages_fetched }}/{{ j.daily_quota }}</td>
            <td class="act" @click.stop>
              <button @click="action(j.id, 'pause')">Pause</button>
              <button @click="action(j.id, 'resume')">Resume</button>
              <button @click="action(j.id, 'cancel')">Cancel</button>
              <button @click="action(j.id, 'reset')">Reset</button>
            </td>
          </tr>
          <tr v-if="!jobs.length"><td colspan="6" class="empty">No jobs</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel" v-if="metrics">
      <h3>Job #{{ metrics.job_id }} · {{ metrics.status }} · {{ metrics.duration_ms }} ms</h3>
      <ul>
        <li v-for="m in metrics.metrics" :key="m.name">
          {{ m.name }}: <strong>{{ m.total }}</strong> {{ m.unit }} ({{ m.samples }} samples)
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.crawl { display: flex; flex-direction: column; gap: 1em; }
.panel { background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 1em; }
.panel h3 { margin: 0 0 0.5em; }
table { width: 100%; border-collapse: collapse; font-size: 0.9em; }
th, td { text-align: left; padding: 0.4em 0.5em; border-bottom: 1px solid #f3f4f6; }
th { color: #6b7280; font-weight: 600; }
tr.sel { background: #eff6ff; }
tr { cursor: pointer; }
.online, .completed { color: #059669; }
.offline, .failed, .cancelled { color: #b91c1c; }
.running { color: #2563eb; }
.paused, .queued { color: #ca8a04; }
.form { display: flex; gap: 0.5em; flex-wrap: wrap; }
.form input { padding: 0.4em 0.6em; border: 1px solid #ccc; border-radius: 4px; flex: 1; min-width: 10em; }
.form button { padding: 0.4em 1em; border: 1px solid #2563eb; background: #2563eb; color: white; border-radius: 4px; cursor: pointer; }
.act button { font-size: 0.75em; padding: 0.15em 0.45em; margin-right: 0.25em; border: 1px solid #d1d5db; background: #fff; border-radius: 3px; cursor: pointer; }
.err { color: #b91c1c; margin: 0.5em 0 0; }
.empty { color: #9ca3af; text-align: center; }
ul { padding: 0; margin: 0; list-style: none; }
ul li { font-size: 0.9em; padding: 0.25em 0; }
</style>
