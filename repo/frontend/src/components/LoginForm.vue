<script setup>
import { ref } from 'vue'
import { login, useAuth } from '../composables/useAuth.js'

const { loginError } = useAuth()

const username = ref('')
const password = ref('')
const busy = ref(false)

async function onSubmit() {
  busy.value = true
  try { await login(username.value, password.value) }
  finally { busy.value = false }
}
</script>

<template>
  <form class="login" @submit.prevent="onSubmit">
    <h2>Sign in</h2>
    <label>
      <span>Username</span>
      <input v-model="username" autocomplete="username" required autofocus />
    </label>
    <label>
      <span>Password</span>
      <input v-model="password" type="password" autocomplete="current-password" required />
    </label>
    <button type="submit" :disabled="busy">
      {{ busy ? 'Signing in…' : 'Sign in' }}
    </button>
    <p v-if="loginError" class="err">{{ loginError }}</p>
    <p class="hint">Default admin: <code>admin</code> / <code>admin123</code></p>
  </form>
</template>

<style scoped>
.login {
  max-width: 22em;
  margin: 3em auto;
  padding: 1.5em;
  border: 1px solid #ddd;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 0.75em;
}
.login h2 { margin: 0 0 0.5em; }
label { display: flex; flex-direction: column; gap: 0.25em; font-size: 0.9em; }
input { padding: 0.4em 0.6em; font-size: 1em; border: 1px solid #ccc; border-radius: 4px; }
button { padding: 0.5em; font-size: 1em; cursor: pointer; }
button[disabled] { opacity: 0.5; cursor: wait; }
.err { color: #b91c1c; font-size: 0.85em; margin: 0; }
.hint { color: #666; font-size: 0.8em; margin: 0; }
code { background: #f3f4f6; padding: 0.1em 0.3em; border-radius: 3px; }
</style>
