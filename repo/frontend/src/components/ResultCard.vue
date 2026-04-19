<script setup>
import HighlightedText from './HighlightedText.vue'

defineProps({
  hit: { type: Object, required: true },
})
</script>

<template>
  <article class="card">
    <header>
      <h3>
        <HighlightedText :text="hit.title_highlighted" :fallback="hit.title" />
      </h3>
      <div class="meta">
        <span class="score">score {{ hit.score?.toFixed?.(2) ?? hit.score }}</span>
        <span v-if="hit.matched_fields?.length" class="fields">
          matched: {{ hit.matched_fields.join(', ') }}
        </span>
      </div>
    </header>
    <p v-if="hit.first_line_highlighted || hit.first_line" class="first-line">
      <HighlightedText :text="hit.first_line_highlighted" :fallback="hit.first_line" />
    </p>
    <p v-if="hit.snippet" class="snippet">
      <HighlightedText :text="hit.snippet" />
    </p>
  </article>
</template>

<style scoped>
.card {
  padding: 0.85em 1em;
  border: 1px solid #eee;
  border-radius: 6px;
  background: #fff;
}
h3 { margin: 0 0 0.25em; font-size: 1.05em; }
.meta { font-size: 0.75em; color: #666; display: flex; gap: 0.75em; }
.score { color: #2563eb; }
.first-line { margin: 0.4em 0 0; color: #222; white-space: pre-wrap; }
.snippet {
  margin: 0.4em 0 0;
  color: #555;
  font-size: 0.9em;
  padding: 0.4em 0.6em;
  background: #fafafa;
  border-left: 3px solid #ddd;
  white-space: pre-wrap;
}
</style>
