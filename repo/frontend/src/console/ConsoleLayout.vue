<script setup>
import SideNav from './SideNav.vue'
import { useAuth } from '../composables/useAuth.js'

const { user } = useAuth()
</script>

<template>
  <div class="console">
    <SideNav />
    <div class="main">
      <header class="ph">
        <h2>{{ $route.meta?.title || $route.name?.toString().replace('console.', '').replace(/^\w/, c => c.toUpperCase()) }}</h2>
        <span class="role">{{ user?.username }} · {{ user?.role }}</span>
      </header>
      <section class="body">
        <router-view />
      </section>
    </div>
  </div>
</template>

<style scoped>
.console {
  display: grid;
  grid-template-columns: auto 1fr;
  min-height: calc(100vh - 3.5em);
}
.main { display: flex; flex-direction: column; }
.ph {
  padding: 1em 1.5em;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.ph h2 { margin: 0; font-size: 1.1em; color: #111827; }
.role { color: #6b7280; font-size: 0.85em; }
.body { padding: 1.25em 1.5em; flex: 1; overflow: auto; }
</style>
