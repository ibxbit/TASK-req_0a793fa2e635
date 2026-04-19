<script setup>
import { ref, watch } from 'vue'
import { get, post, del } from '../api.js'

const tabs = [
  { key: 'poems',     label: 'Poems',     url: '/poems',     create: { title: '', body: '' } },
  { key: 'authors',   label: 'Authors',   url: '/authors',   create: { name: '' } },
  { key: 'dynasties', label: 'Dynasties', url: '/dynasties', create: { name: '' } },
  { key: 'tags',      label: 'Tags',      url: '/tags',      create: { name: '' } },
]
const active = ref(tabs[0])
const items  = ref([])
const form   = ref({ ...tabs[0].create })
const busy   = ref(false)
const error  = ref('')

async function load() {
  error.value = ''
  try {
    const data = await get(active.value.url, { limit: 50 })
    items.value = data.items || []
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
    items.value = []
  }
}

watch(active, () => { form.value = { ...active.value.create }; load() }, { immediate: true })

async function onCreate() {
  busy.value = true
  error.value = ''
  try {
    await post(active.value.url, form.value)
    form.value = { ...active.value.create }
    await load()
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
  } finally {
    busy.value = false
  }
}

async function onDelete(id) {
  if (!confirm(`Delete ${active.value.label.toLowerCase()} #${id}?`)) return
  try { await del(`${active.value.url}/${id}`); await load() }
  catch (e) { error.value = e?.response?.data?.error || e.message }
}

function preview(item) {
  if (item.title) return item.title
  if (item.name)  return item.name
  return `#${item.id}`
}
</script>

<template>
  <div>
    <nav class="tabs">
      <button
        v-for="t in tabs"
        :key="t.key"
        :class="{ on: active.key === t.key }"
        @click="active = t"
      >{{ t.label }}</button>
    </nav>

    <form class="create" @submit.prevent="onCreate">
      <template v-for="(_, key) in active.create" :key="key">
        <input :placeholder="key" v-model="form[key]" />
      </template>
      <button :disabled="busy" type="submit">{{ busy ? '…' : 'Create' }}</button>
    </form>

    <p v-if="error" class="err">{{ error }}</p>

    <ul class="list">
      <li v-for="it in items" :key="it.id">
        <span class="id">#{{ it.id }}</span>
        <span class="t">{{ preview(it) }}</span>
        <button class="del" @click="onDelete(it.id)">Delete</button>
      </li>
      <li v-if="!items.length" class="empty">No items.</li>
    </ul>
  </div>
</template>

<style scoped>
.tabs { display: flex; gap: 0.25em; margin-bottom: 1em; }
.tabs button {
  padding: 0.4em 0.9em;
  border: 1px solid #d1d5db;
  background: #fff;
  cursor: pointer;
  border-radius: 4px;
}
.tabs button.on { background: #1f2937; color: #fff; border-color: #1f2937; }
.create { display: flex; gap: 0.5em; margin-bottom: 1em; flex-wrap: wrap; }
.create input { padding: 0.4em 0.6em; border: 1px solid #ccc; border-radius: 4px; font-size: 0.9em; flex: 1; min-width: 10em; }
.create button { padding: 0.4em 1em; border-radius: 4px; border: 1px solid #2563eb; background: #2563eb; color: white; cursor: pointer; }
.err { color: #b91c1c; font-size: 0.9em; }
.list { list-style: none; padding: 0; margin: 0; background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; }
.list li { display: flex; align-items: center; gap: 0.75em; padding: 0.5em 0.75em; border-bottom: 1px solid #f3f4f6; }
.list li:last-child { border-bottom: 0; }
.id { color: #9ca3af; font-size: 0.8em; min-width: 3em; }
.t { flex: 1; color: #111827; }
.del { font-size: 0.8em; padding: 0.2em 0.6em; border: 1px solid #fecaca; background: #fee2e2; color: #991b1b; border-radius: 4px; cursor: pointer; }
.empty { color: #6b7280; }
</style>
