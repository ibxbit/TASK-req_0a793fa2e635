<script setup>
import { isOnline } from '../offline/network.js'
</script>

<template>
  <span class="net" :class="{ online: isOnline, offline: !isOnline }">
    <span class="dot"></span>
    <span class="label">{{ isOnline ? 'Online' : 'Offline' }}</span>
  </span>
</template>

<style scoped>
.net {
  display: inline-flex;
  align-items: center;
  gap: 0.4em;
  font-size: 0.85em;
  padding: 0.15em 0.55em;
  border-radius: 999px;
  border: 1px solid #ccc;
}
.dot {
  width: 0.55em;
  height: 0.55em;
  border-radius: 50%;
  background: #999;
}
.online { border-color: #0a7a2f; color: #0a7a2f; }
.online .dot { background: #0a7a2f; }
.offline { border-color: #b91c1c; color: #b91c1c; }
.offline .dot { background: #b91c1c; }
</style>
