<script setup>
import SearchBar from '../components/SearchBar.vue'
import FilterPanel from '../components/FilterPanel.vue'
import ResultsList from '../components/ResultsList.vue'
</script>

<template>
  <div class="search-page">
    <SearchBar />
    <div class="layout">
      <FilterPanel />
      <ResultsList />
    </div>
  </div>
</template>

<style scoped>
.search-page { padding: 1em 0; }
.layout {
  display: grid;
  grid-template-columns: 16em 1fr;
  gap: 1.5em;
  margin-top: 1em;
  align-items: start;
}
@media (max-width: 720px) {
  .layout { grid-template-columns: 1fr; }
}
</style>
