<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import {
  listQueue,
  processQueue,
  removeFromQueue,
  retryEntry,
  queueState,
} from '../offline/queue.js'

const open = ref(false)
const items = ref([])

async function refresh() {
  items.value = (await listQueue()).sort((a, b) => a.created_at - b.created_at)
}

let timer = null

onMounted(async () => {
  await refresh()
  timer = setInterval(refresh, 2000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })

async function retryAll() {
  await processQueue()
  await refresh()
}

async function retryOne(id) {
  await retryEntry(id)
  await refresh()
}

async function drop(id) {
  await removeFromQueue(id)
  await refresh()
}

function relTime(ts) {
  const diff = Math.max(0, ts - Date.now())
  if (diff === 0) return 'now'
  const s = Math.round(diff / 1000)
  return `in ${s}s`
}
</script>

<template>
  <div class="queue-wrap">
    <button class="queue-toggle" @click="open = !open">
      Queue
      <span class="badge" v-if="queueState.count > 0">{{ queueState.count }}</span>
    </button>
    <div v-if="open" class="queue-panel">
      <div class="head">
        <strong>Queued Actions</strong>
        <div>
          <button @click="retryAll">Retry now</button>
          <button @click="open = false">Close</button>
        </div>
      </div>
      <p class="summary">
        {{ queueState.count }} total · {{ queueState.pending }} pending · {{ queueState.failed }} failed
      </p>
      <ul v-if="items.length">
        <li v-for="i in items" :key="i.id" :class="i.status">
          <div class="row">
            <code>{{ i.method }} {{ i.url }}</code>
            <span class="status">{{ i.status }}</span>
          </div>
          <div class="meta">
            <span>kind: {{ i.kind }}</span>
            <span>retries: {{ i.retries }}</span>
            <span v-if="i.status === 'pending'">next: {{ relTime(i.next_retry_at) }}</span>
          </div>
          <div class="err" v-if="i.error">{{ i.error }}</div>
          <div class="actions">
            <button @click="retryOne(i.id)" v-if="i.status !== 'in_flight'">Retry</button>
            <button @click="drop(i.id)">Drop</button>
          </div>
        </li>
      </ul>
      <p v-else class="empty">No queued actions.</p>
    </div>
  </div>
</template>

<style scoped>
.queue-wrap { position: relative; display: inline-block; }
.queue-toggle {
  padding: 0.3em 0.8em;
  border: 1px solid #ccc;
  background: #fff;
  border-radius: 4px;
  cursor: pointer;
}
.badge {
  display: inline-block;
  margin-left: 0.4em;
  min-width: 1.3em;
  padding: 0 0.4em;
  background: #b91c1c;
  color: white;
  border-radius: 999px;
  font-size: 0.75em;
  text-align: center;
}
.queue-panel {
  position: absolute;
  top: 2.2em;
  right: 0;
  width: 28em;
  max-height: 60vh;
  overflow-y: auto;
  background: white;
  border: 1px solid #ccc;
  border-radius: 4px;
  padding: 0.75em;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  z-index: 20;
}
.head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5em; }
.summary { margin: 0 0 0.5em 0; font-size: 0.85em; color: #555; }
ul { list-style: none; padding: 0; margin: 0; }
li { padding: 0.5em 0; border-bottom: 1px solid #eee; }
.row { display: flex; justify-content: space-between; font-size: 0.9em; }
code { font-family: ui-monospace, monospace; font-size: 0.85em; }
.status { font-weight: bold; }
li.failed .status { color: #b91c1c; }
li.pending .status { color: #ca8a04; }
li.in_flight .status { color: #2563eb; }
.meta { font-size: 0.75em; color: #666; margin-top: 0.2em; display: flex; gap: 0.8em; }
.err { font-size: 0.8em; color: #b91c1c; margin-top: 0.25em; word-break: break-all; }
.actions { margin-top: 0.35em; display: flex; gap: 0.4em; }
.actions button { font-size: 0.8em; }
.empty { color: #666; font-size: 0.85em; }
</style>
