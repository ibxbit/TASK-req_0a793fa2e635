<script setup>
import { computed } from 'vue'
import { useRbac, ROLES } from '../composables/useRbac.js'

const { hasAny } = useRbac()

const items = computed(() => [
  { to: { name: 'console.dashboard' },    label: 'Dashboard',        show: true },
  { to: { name: 'console.content' },      label: 'Content',          show: hasAny(ROLES.ADMIN, ROLES.EDITOR) },
  { to: { name: 'console.pricing' },      label: 'Pricing (Quote)',  show: hasAny(ROLES.ADMIN, ROLES.MKT) },
  { to: { name: 'console.pricing_mgmt' }, label: 'Pricing Management', show: hasAny(ROLES.ADMIN, ROLES.MKT) },
  { to: { name: 'console.complaints' },   label: 'Complaints',       show: hasAny(ROLES.ADMIN, ROLES.REVIEWER) },
  { to: { name: 'console.crawl' },        label: 'Crawl',            show: hasAny(ROLES.ADMIN, ROLES.CRAWLER) },
  { to: { name: 'console.approvals' },    label: 'Approvals',        show: hasAny(ROLES.ADMIN) },
  { to: { name: 'console.revisions' },    label: 'Revisions',        show: hasAny(ROLES.ADMIN) },
  { to: { name: 'console.audit' },        label: 'Audit Logs',       show: hasAny(ROLES.ADMIN) },
  { to: { name: 'console.monitoring' },   label: 'Monitoring',       show: hasAny(ROLES.ADMIN) },
  { to: { name: 'console.settings' },     label: 'Settings',         show: hasAny(ROLES.ADMIN) },
].filter(i => i.show))
</script>

<template>
  <nav class="side-nav">
    <router-link
      v-for="it in items"
      :key="it.label"
      :to="it.to"
      class="item"
      active-class="active"
    >
      {{ it.label }}
    </router-link>
    <div class="sep"></div>
    <router-link to="/" class="item subtle">← Public search</router-link>
  </nav>
</template>

<style scoped>
.side-nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 0.75em;
  background: #fff;
  border-right: 1px solid #e5e7eb;
  min-width: 14em;
}
.item {
  padding: 0.5em 0.75em;
  color: #374151;
  text-decoration: none;
  border-radius: 4px;
  font-size: 0.95em;
}
.item:hover { background: #f3f4f6; }
.active { background: #1f2937; color: #fff; }
.active:hover { background: #111827; }
.sep { height: 1px; background: #eee; margin: 0.75em 0; }
.subtle { color: #6b7280; font-size: 0.85em; }
</style>
