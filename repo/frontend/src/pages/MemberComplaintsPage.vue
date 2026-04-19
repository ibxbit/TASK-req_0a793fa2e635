<script setup>
import { ref, onMounted } from 'vue'
import { apiGet } from '../offline/api.js'

const items = ref([])
const loading = ref(false)
const error = ref('')
const fromCache = ref(false)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await apiGet('/api/v1/complaints/mine', {
      cacheKey: 'complaints:mine',
      ttlMs: 60_000,
    })
    items.value = res.data.items || []
    fromCache.value = res.fromCache
  } catch (e) {
    error.value = e?.response?.data?.error || e?.message || 'failed to load complaints'
    items.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)

function fmt(ts) {
  if (!ts) return '—'
  try { return new Date(ts).toLocaleString() } catch (_) { return ts }
}
</script>

<template>
  <div class="my-complaints">
    <h2>My Complaints</h2>

    <p v-if="fromCache" class="cache-note" data-test="cache-note">
      Showing cached data — you may be offline.
    </p>

    <div v-if="loading" class="loading" data-test="loading">Loading…</div>

    <p v-else-if="error" class="error" role="alert" data-test="error">{{ error }}</p>

    <p v-else-if="!items.length" class="empty" data-test="empty">
      You have not submitted any complaints yet.
    </p>

    <ul v-else class="list" data-test="list">
      <li v-for="c in items" :key="c.id" class="item" :data-id="c.id">
        <div class="subject">{{ c.subject }}</div>
        <div class="meta">
          <span class="badge target">{{ c.target_type }}</span>
          <span class="badge status" :class="'s-' + c.arbitration_code">{{ c.arbitration_code }}</span>
        </div>
        <div v-if="c.resolution" class="detail">Resolution: {{ c.resolution }}</div>
        <div v-if="c.resolved_at" class="detail">Resolved: {{ fmt(c.resolved_at) }}</div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.my-complaints { max-width: 42em; padding: 1.5em 0; }
h2 { margin-top: 0; }
.loading { color: #6b7280; padding: 0.5em 0; }
.error { color: #dc2626; }
.empty { color: #6b7280; font-style: italic; }
.cache-note { color: #b45309; font-size: 0.85em; margin-bottom: 0.5em; }
.list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 0.75em; }
.item {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 0.75em 1em;
  display: flex;
  flex-direction: column;
  gap: 0.3em;
}
.subject { font-weight: 600; color: #111827; }
.meta { display: flex; gap: 0.5em; align-items: center; flex-wrap: wrap; }
.badge {
  font-size: 0.78em;
  font-weight: 500;
  padding: 0.15em 0.55em;
  border-radius: 4px;
}
.target { background: #f3f4f6; color: #374151; }
.status { background: #fef9c3; color: #854d0e; }
.s-resolved_upheld, .s-resolved_dismissed { background: #d1fae5; color: #065f46; }
.detail { font-size: 0.85em; color: #6b7280; }
</style>
