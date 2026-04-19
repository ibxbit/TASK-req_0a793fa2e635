<script setup>
import { useSearch } from '../composables/useSearch.js'

const { query, options, loading } = useSearch()
</script>

<template>
  <div class="search-bar">
    <input
      v-model="query"
      class="q"
      type="search"
      placeholder="Search poems, authors, lines…"
      autofocus
    />
    <div class="toggles">
      <label>
        <input type="checkbox" v-model="options.highlight" />
        Highlight
      </label>
      <label>
        <input type="checkbox" v-model="options.syn" />
        Synonyms
      </label>
      <label>
        <input type="checkbox" v-model="options.cjk" />
        SC ↔ TC
      </label>
    </div>
    <div class="status">
      <span v-if="loading">Searching…</span>
    </div>
  </div>
</template>

<style scoped>
.search-bar {
  display: flex;
  align-items: center;
  gap: 1em;
  flex-wrap: wrap;
  padding: 0.5em 0;
}
.q {
  flex: 1 1 20em;
  min-width: 12em;
  font-size: 1.1em;
  padding: 0.5em 0.75em;
  border: 1px solid #ccc;
  border-radius: 6px;
}
.toggles {
  display: flex;
  gap: 0.75em;
  font-size: 0.9em;
  color: #444;
}
.toggles label { display: inline-flex; align-items: center; gap: 0.25em; }
.status { font-size: 0.85em; color: #666; min-width: 6em; }
</style>
