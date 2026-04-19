<script setup>
import { onMounted } from 'vue'
import { useSearch } from '../composables/useSearch.js'
import { useFilters, loadFilters } from '../composables/useFilters.js'

const { filters } = useSearch()
const { authors, dynasties, tags, loading } = useFilters()

onMounted(() => { loadFilters() })

function clearAll() {
  filters.author_id = null
  filters.dynasty_id = null
  filters.tag_id = null
  filters.meter_id = null
  filters.snippet = ''
}
</script>

<template>
  <aside class="filters">
    <header>
      <strong>Filters</strong>
      <button @click="clearAll" class="clear">Clear</button>
    </header>
    <p v-if="loading" class="hint">Loading options…</p>

    <label>
      <span>Author</span>
      <select v-model.number="filters.author_id">
        <option :value="null">Any</option>
        <option v-for="a in authors" :key="a.id" :value="a.id">{{ a.name }}</option>
      </select>
    </label>

    <label>
      <span>Dynasty</span>
      <select v-model.number="filters.dynasty_id">
        <option :value="null">Any</option>
        <option v-for="d in dynasties" :key="d.id" :value="d.id">{{ d.name }}</option>
      </select>
    </label>

    <label>
      <span>Tag</span>
      <select v-model.number="filters.tag_id">
        <option :value="null">Any</option>
        <option v-for="t in tags" :key="t.id" :value="t.id">{{ t.name }}</option>
      </select>
    </label>

    <label>
      <span>Meter ID</span>
      <input type="number" min="1" v-model.number="filters.meter_id" placeholder="e.g. 3" />
    </label>

    <label>
      <span>Line contains</span>
      <input type="text" v-model="filters.snippet" placeholder="Substring match" />
    </label>
  </aside>
</template>

<style scoped>
.filters {
  width: 16em;
  min-width: 14em;
  padding: 1em;
  background: #fafafa;
  border: 1px solid #eee;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 0.75em;
}
header { display: flex; justify-content: space-between; align-items: center; }
.clear { font-size: 0.8em; padding: 0.2em 0.5em; cursor: pointer; }
label { display: flex; flex-direction: column; gap: 0.2em; font-size: 0.85em; color: #444; }
select, input {
  padding: 0.35em 0.5em;
  font-size: 0.95em;
  border: 1px solid #ccc;
  border-radius: 4px;
}
.hint { color: #666; font-size: 0.8em; margin: 0; }
</style>
