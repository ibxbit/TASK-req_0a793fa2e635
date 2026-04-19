<script setup>
import { computed } from 'vue'
import ResultCard from './ResultCard.vue'
import DidYouMean from './DidYouMean.vue'
import { useSearch } from '../composables/useSearch.js'

const { results, loading, error, fromCache, pagination, query } = useSearch()

const hasQuery = computed(() => (query.value || '').trim().length > 0)

function nextPage() { pagination.offset += pagination.limit }
function prevPage() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
}

const showingStart = computed(() => (results.value.hits?.length ? pagination.offset + 1 : 0))
const showingEnd = computed(() => pagination.offset + (results.value.hits?.length || 0))
</script>

<template>
  <section class="results">
    <div class="bar">
      <span v-if="results.count > 0">
        Showing {{ showingStart }}–{{ showingEnd }} of {{ results.count }} this page
      </span>
      <span v-else-if="!loading && hasQuery">No results</span>
      <span v-else-if="!hasQuery">Start typing to search</span>
      <span v-if="fromCache" class="tag">cached</span>
    </div>

    <DidYouMean :suggestions="results.did_you_mean" />

    <p v-if="error" class="error">{{ error }}</p>

    <div class="list">
      <ResultCard v-for="h in results.hits" :key="h.poem_id" :hit="h" />
    </div>

    <div class="pager" v-if="results.count > 0 || pagination.offset > 0">
      <button :disabled="pagination.offset === 0 || loading" @click="prevPage">← Prev</button>
      <span class="page">offset {{ pagination.offset }}</span>
      <button :disabled="(results.hits?.length || 0) < pagination.limit || loading" @click="nextPage">Next →</button>
    </div>
  </section>
</template>

<style scoped>
.results { display: flex; flex-direction: column; gap: 0.75em; }
.bar {
  display: flex;
  align-items: center;
  gap: 0.75em;
  color: #555;
  font-size: 0.9em;
}
.tag {
  background: #e0e7ff;
  color: #3730a3;
  font-size: 0.75em;
  padding: 0.15em 0.5em;
  border-radius: 999px;
}
.error {
  padding: 0.75em;
  background: #fee2e2;
  color: #991b1b;
  border-radius: 4px;
  margin: 0;
}
.list { display: flex; flex-direction: column; gap: 0.6em; }
.pager {
  display: flex;
  align-items: center;
  gap: 0.75em;
  margin-top: 0.5em;
}
.pager button {
  padding: 0.3em 0.8em;
  border: 1px solid #ccc;
  background: #fff;
  border-radius: 4px;
  cursor: pointer;
}
.pager button[disabled] { opacity: 0.5; cursor: not-allowed; }
.page { color: #666; font-size: 0.85em; }
</style>
