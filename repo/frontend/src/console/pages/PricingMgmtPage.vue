<script setup>
// Pricing management console — admin + marketing_manager.
//
// Four tabs, each a minimal list + "new" form wired against the
// /campaigns, /coupons, /pricing-rules, /member-tiers endpoints.
// Every mutation carries an Idempotency-Key so reloads after a PUT/POST
// won't double-apply on the server.
import { ref, watch, computed } from 'vue'
import { get, post, put, del } from '../api.js'

const tabs = [
  {
    key: 'campaigns', label: 'Campaigns', url: '/campaigns',
    columns: ['name', 'campaign_type', 'discount_type', 'discount_value', 'status'],
    blank: () => ({
      name: '', campaign_type: 'standard', discount_type: 'percentage',
      discount_value: 10, status: 'draft', description: '',
    }),
  },
  {
    key: 'coupons', label: 'Coupons', url: '/coupons',
    columns: ['code', 'discount_type', 'discount_value', 'used_count', 'usage_limit', 'status'],
    blank: () => ({
      code: '', discount_type: 'percentage', discount_value: 10, status: 'active',
      usage_limit: null, per_user_limit: null,
    }),
  },
  {
    key: 'rules', label: 'Pricing Rules', url: '/pricing-rules',
    columns: ['name', 'rule_type', 'target_scope', 'value', 'priority', 'active'],
    blank: () => ({
      name: '', rule_type: 'percentage', target_scope: 'all',
      value: 5, priority: 0, active: true,
    }),
  },
  {
    key: 'tiers', label: 'Member Tiers', url: '/member-tiers',
    columns: ['name', 'level', 'monthly_price', 'yearly_price'],
    blank: () => ({ name: '', level: 0, monthly_price: 0, yearly_price: 0 }),
  },
]

const active = ref(tabs[0])
const items  = ref([])
const form   = ref(tabs[0].blank())
const busy   = ref(false)
const error  = ref('')

async function load() {
  error.value = ''
  try {
    const data = await get(active.value.url, { limit: 200 })
    items.value = data.items || []
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
    items.value = []
  }
}

watch(active, () => {
  form.value = active.value.blank()
  load()
}, { immediate: true })

async function onCreate() {
  busy.value = true
  error.value = ''
  try {
    await post(active.value.url, sanitize(form.value))
    form.value = active.value.blank()
    await load()
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
  } finally {
    busy.value = false
  }
}

async function onDelete(id) {
  if (!confirm(`Delete ${active.value.label.toLowerCase().slice(0, -1)} #${id}?`)) return
  try {
    await del(`${active.value.url}/${id}`)
    await load()
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
  }
}

async function onToggleActive(row) {
  if (active.value.key !== 'rules') return
  try {
    await put(`${active.value.url}/${row.id}`, { ...row, active: !row.active })
    await load()
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
  }
}

// The form binds `null`-able numeric fields as strings; coerce them back to
// numbers (or strip them) before sending so the API doesn't 400.
function sanitize(obj) {
  const out = { ...obj }
  for (const k of Object.keys(out)) {
    if (out[k] === '' || out[k] === null) delete out[k]
    else if (typeof out[k] === 'string' && !isNaN(Number(out[k])) && /_value$|_price$|_limit$|level|priority/.test(k)) {
      out[k] = Number(out[k])
    }
  }
  return out
}

const placeholderFor = (key) => key.replace(/_/g, ' ')
const visibleFields = computed(() => Object.keys(active.value.blank()))

function formatCell(row, col) {
  const v = row[col]
  if (v === null || v === undefined) return '—'
  if (typeof v === 'boolean') return v ? 'yes' : 'no'
  return v
}
</script>

<template>
  <div>
    <nav class="tabs">
      <button
        v-for="t in tabs"
        :key="t.key"
        :class="{ on: active.key === t.key }"
        @click="active = t"
      >{{ t.label }}</button>
    </nav>

    <form class="create" @submit.prevent="onCreate">
      <input
        v-for="f in visibleFields"
        :key="f"
        :placeholder="placeholderFor(f)"
        v-model="form[f]"
      />
      <button :disabled="busy" type="submit">{{ busy ? '…' : 'Create' }}</button>
    </form>

    <p v-if="error" class="err" data-test="pm-error">{{ error }}</p>

    <table class="list" v-if="items.length">
      <thead>
        <tr>
          <th>#</th>
          <th v-for="col in active.columns" :key="col">{{ col }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="it in items" :key="it.id">
          <td class="id">{{ it.id }}</td>
          <td v-for="col in active.columns" :key="col">{{ formatCell(it, col) }}</td>
          <td class="actions">
            <button v-if="active.key === 'rules'" @click="onToggleActive(it)" class="toggle">
              {{ it.active ? 'Disable' : 'Enable' }}
            </button>
            <button class="del" @click="onDelete(it.id)">Delete</button>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-else class="empty">No {{ active.label.toLowerCase() }} yet.</p>
  </div>
</template>

<style scoped>
.tabs { display: flex; gap: 0.25em; margin-bottom: 1em; flex-wrap: wrap; }
.tabs button {
  padding: 0.4em 0.9em;
  border: 1px solid #d1d5db;
  background: #fff;
  cursor: pointer;
  border-radius: 4px;
}
.tabs button.on { background: #1f2937; color: #fff; border-color: #1f2937; }
.create { display: flex; gap: 0.4em; margin-bottom: 1em; flex-wrap: wrap; }
.create input {
  padding: 0.4em 0.6em;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 0.9em;
  flex: 1;
  min-width: 9em;
}
.create button {
  padding: 0.4em 1em;
  border-radius: 4px;
  border: 1px solid #2563eb;
  background: #2563eb;
  color: white;
  cursor: pointer;
}
.err { color: #b91c1c; font-size: 0.9em; }
.list {
  width: 100%;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  border-collapse: collapse;
}
.list th, .list td {
  padding: 0.5em 0.75em;
  text-align: left;
  border-bottom: 1px solid #f3f4f6;
  font-size: 0.9em;
}
.list th { background: #f9fafb; }
.id { color: #9ca3af; width: 3em; }
.actions { display: flex; gap: 0.3em; justify-content: flex-end; }
.del {
  font-size: 0.8em;
  padding: 0.2em 0.6em;
  border: 1px solid #fecaca;
  background: #fee2e2;
  color: #991b1b;
  border-radius: 4px;
  cursor: pointer;
}
.toggle {
  font-size: 0.8em;
  padding: 0.2em 0.6em;
  border: 1px solid #bfdbfe;
  background: #eff6ff;
  color: #1e40af;
  border-radius: 4px;
  cursor: pointer;
}
.empty { color: #6b7280; font-size: 0.9em; }
</style>
