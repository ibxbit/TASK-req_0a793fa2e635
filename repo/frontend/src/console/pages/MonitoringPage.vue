<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { get } from '../api.js'

const gauges  = ref([])
const crashes = ref([])
const error   = ref('')

async function load() {
  try { gauges.value  = (await get('/monitoring/metrics/summary')).items || [] } catch {}
  try { crashes.value = (await get('/monitoring/crashes')).items || [] }
  catch (e) { error.value = e?.response?.data?.error || e.message }
}

let timer
onMounted(() => { load(); timer = setInterval(load, 15_000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="mon">
    <section class="panel">
      <h3>Runtime gauges</h3>
      <div class="grid">
        <div v-for="g in gauges" :key="g.name" class="tile">
          <div class="n">{{ g.name }}</div>
          <div class="v">{{ Number(g.value).toLocaleString() }} <span class="u">{{ g.unit }}</span></div>
          <div class="t">{{ new Date(g.recorded_at).toLocaleTimeString() }}</div>
        </div>
        <div v-if="!gauges.length" class="empty">No samples yet — sampler runs every 30 s.</div>
      </div>
    </section>

    <section class="panel">
      <h3>Recent crashes</h3>
      <p v-if="error" class="err">{{ error }}</p>
      <table v-if="crashes.length">
        <thead><tr><th>ID</th><th>When</th><th>Service</th><th>Type</th><th>Message</th></tr></thead>
        <tbody>
          <tr v-for="c in crashes" :key="c.id">
            <td>{{ c.id }}</td>
            <td>{{ new Date(c.occurred_at).toLocaleString() }}</td>
            <td>{{ c.service }}</td>
            <td>{{ c.error_type }}</td>
            <td class="msg">{{ c.error_message }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="empty">No crashes recorded.</p>
    </section>
  </div>
</template>

<style scoped>
.mon { display: flex; flex-direction: column; gap: 1em; }
.panel { background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 1em; }
.panel h3 { margin: 0 0 0.5em; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(10em, 1fr)); gap: 0.5em; }
.tile { background: #f9fafb; padding: 0.6em 0.8em; border-radius: 4px; border: 1px solid #f3f4f6; }
.n { font-size: 0.75em; color: #6b7280; text-transform: uppercase; }
.v { font-size: 1.2em; font-weight: 600; color: #111827; }
.u { font-size: 0.7em; color: #6b7280; margin-left: 0.25em; }
.t { font-size: 0.7em; color: #9ca3af; margin-top: 0.25em; }
.empty { color: #9ca3af; font-size: 0.9em; }
.err { color: #b91c1c; }
table { width: 100%; border-collapse: collapse; font-size: 0.85em; }
th, td { padding: 0.3em 0.5em; border-bottom: 1px solid #f3f4f6; text-align: left; }
th { color: #6b7280; }
.msg { font-family: ui-monospace, monospace; font-size: 0.85em; color: #7f1d1d; }
</style>
