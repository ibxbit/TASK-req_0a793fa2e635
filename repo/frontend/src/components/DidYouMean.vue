<script setup>
import { useSearch } from '../composables/useSearch.js'

const props = defineProps({
  suggestions: { type: Array, default: () => [] },
})

const { query } = useSearch()

function pick(term) {
  query.value = term
}
</script>

<template>
  <div v-if="suggestions?.length" class="dym">
    <span class="label">Did you mean:</span>
    <button
      v-for="s in suggestions"
      :key="s.term"
      class="chip"
      @click="pick(s.term)"
      :title="`distance ${s.distance} · source ${s.source}`"
    >
      {{ s.term }}
    </button>
  </div>
</template>

<style scoped>
.dym {
  display: flex;
  align-items: center;
  gap: 0.5em;
  flex-wrap: wrap;
  padding: 0.5em 0.75em;
  background: #fff8e1;
  border: 1px solid #fde68a;
  border-radius: 6px;
  margin-bottom: 1em;
}
.label { font-size: 0.85em; color: #92400e; }
.chip {
  background: white;
  border: 1px solid #fde68a;
  color: #92400e;
  padding: 0.2em 0.6em;
  border-radius: 999px;
  cursor: pointer;
  font-size: 0.85em;
}
.chip:hover { background: #fde68a; }
</style>
