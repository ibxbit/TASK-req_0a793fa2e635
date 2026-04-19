<script setup>
import { ref, onMounted } from 'vue'
import { get, put } from '../api.js'

const enabled = ref(false)
const loaded = ref(false)
const error = ref('')

async function load() {
  try {
    const r = await get('/settings/approval')
    enabled.value = !!r.approval_required
    loaded.value = true
  } catch (e) { error.value = e?.response?.data?.error || e.message }
}

async function toggle() {
  try {
    await put('/settings/approval', { enabled: !enabled.value })
    enabled.value = !enabled.value
  } catch (e) { error.value = e?.response?.data?.error || e.message }
}

onMounted(load)
</script>

<template>
  <div class="settings">
    <div class="card">
      <h3>Approval workflow</h3>
      <p>When enabled, <strong>deletions and bulk edits</strong> require admin approval within 48 h or auto-revert.</p>
      <div class="toggle">
        <span>Require approval:</span>
        <button :class="['switch', enabled ? 'on' : 'off']" @click="toggle" :disabled="!loaded">
          {{ enabled ? 'ON' : 'OFF' }}
        </button>
      </div>
      <p v-if="error" class="err">{{ error }}</p>
    </div>
  </div>
</template>

<style scoped>
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 1em; max-width: 40em; }
.card h3 { margin: 0 0 0.5em; }
.toggle { display: flex; align-items: center; gap: 0.75em; margin-top: 0.75em; }
.switch { padding: 0.4em 1.2em; border-radius: 999px; border: 1px solid; cursor: pointer; font-weight: 600; }
.switch.on { background: #059669; color: white; border-color: #059669; }
.switch.off { background: #f3f4f6; color: #374151; border-color: #d1d5db; }
.err { color: #b91c1c; }
</style>
