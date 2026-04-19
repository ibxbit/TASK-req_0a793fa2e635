<script setup>
import { computed } from 'vue'

const props = defineProps({
  text: { type: String, default: '' },
  fallback: { type: String, default: '' },
})

const parts = computed(() => {
  const s = props.text || props.fallback || ''
  if (!s) return []
  const out = []
  const re = /<mark>([\s\S]*?)<\/mark>/g
  let last = 0, m
  while ((m = re.exec(s)) !== null) {
    if (m.index > last) out.push({ text: s.slice(last, m.index), hl: false })
    out.push({ text: m[1], hl: true })
    last = m.index + m[0].length
  }
  if (last < s.length) out.push({ text: s.slice(last), hl: false })
  return out
})
</script>

<template>
  <span>
    <template v-for="(p, i) in parts" :key="i">
      <mark v-if="p.hl">{{ p.text }}</mark>
      <template v-else>{{ p.text }}</template>
    </template>
  </span>
</template>

<style scoped>
mark {
  background: #fff3a3;
  padding: 0 0.1em;
  border-radius: 2px;
}
</style>
