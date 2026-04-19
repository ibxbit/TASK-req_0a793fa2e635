<script setup>
import { onMounted, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import NetworkIndicator from './components/NetworkIndicator.vue'
import QueueDrawer from './components/QueueDrawer.vue'
import LoginForm from './components/LoginForm.vue'
import { checkSession, logout, useAuth } from './composables/useAuth.js'
import { useSearch } from './composables/useSearch.js'
import { ROLES } from './composables/useRbac.js'

const { user, ready, isAuthenticated } = useAuth()
const { run: runSearch } = useSearch()
const route = useRoute()
const router = useRouter()

onMounted(checkSession)

watch(isAuthenticated, (ok) => { if (ok) runSearch() }, { immediate: true })

const inConsole      = computed(() => route.path.startsWith('/console'))
const inDownload     = computed(() => route.path.startsWith('/download'))
const inReviews      = computed(() => route.path.startsWith('/reviews'))
const inComplaints   = computed(() => route.path.startsWith('/complaints'))
const inMyComplaints = computed(() => route.path.startsWith('/my-complaints'))
// Members don't get a console — they see Search + Download + Reviews + Complaints.
const canSeeConsole  = computed(() => user.value?.role && user.value.role !== ROLES.MEMBER)

async function onLogout() {
  await logout()
  router.push('/')
}
</script>

<template>
  <header class="top">
    <h1>Helios</h1>
    <nav class="top-nav" v-if="isAuthenticated">
      <router-link to="/" :class="{ on: !inConsole && !inDownload && !inReviews && !inComplaints && !inMyComplaints }">Search</router-link>
      <router-link to="/download" :class="{ on: inDownload }">Download</router-link>
      <router-link to="/reviews/new" :class="{ on: inReviews }">Reviews</router-link>
      <router-link to="/complaints/new" :class="{ on: inComplaints }">Complaints</router-link>
      <router-link to="/my-complaints" :class="{ on: inMyComplaints }">My Complaints</router-link>
      <router-link v-if="canSeeConsole" to="/console" :class="{ on: inConsole }">Console</router-link>
    </nav>
    <div class="top-actions">
      <NetworkIndicator />
      <QueueDrawer v-if="isAuthenticated" />
      <template v-if="isAuthenticated">
        <span class="who">{{ user?.username }} · {{ user?.role }}</span>
        <button class="signout" @click="onLogout">Sign out</button>
      </template>
    </div>
  </header>

  <main :class="{ console: inConsole }">
    <template v-if="!ready">
      <p class="loading">Loading…</p>
    </template>
    <template v-else-if="!isAuthenticated">
      <LoginForm />
    </template>
    <template v-else>
      <router-view />
    </template>
  </main>
</template>

<style>
* { box-sizing: border-box; }
body { font-family: system-ui, -apple-system, "Segoe UI", sans-serif; margin: 0; background: #f8f9fb; color: #1f2937; }
.top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1em;
  padding: 0.75em 1.5em;
  background: white;
  border-bottom: 1px solid #e5e7eb;
  position: sticky;
  top: 0;
  z-index: 5;
}
.top h1 { margin: 0; font-size: 1.25em; color: #111827; }
.top-nav { display: flex; gap: 0.5em; }
.top-nav a {
  padding: 0.3em 0.75em;
  color: #374151;
  text-decoration: none;
  border-radius: 4px;
  font-size: 0.9em;
}
.top-nav a:hover { background: #f3f4f6; }
.top-nav a.on { background: #1f2937; color: #fff; }
.top-actions { display: flex; gap: 0.75em; align-items: center; margin-left: auto; }
.who { font-size: 0.85em; color: #555; }
.signout {
  font-size: 0.85em;
  padding: 0.3em 0.7em;
  border: 1px solid #ccc;
  background: #fff;
  border-radius: 4px;
  cursor: pointer;
}
main { max-width: 72em; margin: 0 auto; padding: 1em 1.5em 3em; }
main.console { max-width: none; padding: 0; }
.loading { color: #666; padding: 1em 1.5em; }
</style>
