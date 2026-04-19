<script setup>
import { ref, onMounted } from 'vue'
import { useRbac, ROLES } from '../../composables/useRbac.js'
import { get } from '../api.js'

const { hasAny } = useRbac()
const health = ref({ status: '—', db: '—' })
const stats = ref({})

async function load() {
  try { health.value = (await get('/health')) } catch {}
  try {
    const nodes = await get('/crawl/nodes').catch(() => ({ items: [] }))
    stats.value.nodes = nodes.items?.length || 0
  } catch {}
  if (hasAny(ROLES.ADMIN)) {
    try {
      const pending = await get('/approvals').catch(() => ({ items: [] }))
      stats.value.pending_approvals = pending.items?.length || 0
    } catch {}
  }
}
onMounted(load)
</script>

<template>
  <div class="dash">
    <div class="card">
      <h3>Backend</h3>
      <p>Status: <strong>{{ health.status }}</strong></p>
      <p>DB: <strong>{{ health.db }}</strong></p>
    </div>
    <div class="card" v-if="stats.nodes != null">
      <h3>Crawl Nodes</h3>
      <p class="big">{{ stats.nodes }}</p>
    </div>
    <div class="card" v-if="stats.pending_approvals != null">
      <h3>Pending Approvals</h3>
      <p class="big">{{ stats.pending_approvals }}</p>
    </div>
  </div>
</template>

<style scoped>
.dash { display: grid; grid-template-columns: repeat(auto-fit, minmax(14em, 1fr)); gap: 1em; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 1em; }
.card h3 { margin: 0 0 0.35em; font-size: 0.9em; color: #6b7280; text-transform: uppercase; }
.big { font-size: 1.8em; font-weight: 600; margin: 0; color: #111827; }
</style>
