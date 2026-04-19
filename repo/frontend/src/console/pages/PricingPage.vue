<script setup>
import { ref, reactive } from 'vue'
import { post } from '../api.js'

const form = reactive({
  coupon_code: '',
  campaign_id: null,
  group_size: 1,
  items: [
    { sku: 'poem:1', price: 50, quantity: 1, member_priced: false },
  ],
})
const quote  = ref(null)
const error  = ref('')
const busy   = ref(false)

function addItem() {
  form.items.push({ sku: '', price: 0, quantity: 1, member_priced: false })
}
function removeItem(i) { form.items.splice(i, 1) }

async function run() {
  busy.value = true
  error.value = ''
  try {
    quote.value = await post('/pricing/quote', {
      coupon_code: form.coupon_code || undefined,
      campaign_id: form.campaign_id || undefined,
      group_size:  Number(form.group_size) || 1,
      items:       form.items.map(i => ({
        sku: i.sku, price: Number(i.price), quantity: Number(i.quantity),
        member_priced: !!i.member_priced,
      })),
    })
  } catch (e) {
    error.value = e?.response?.data?.error || e.message
  } finally { busy.value = false }
}
</script>

<template>
  <div class="pricing">
    <form class="panel" @submit.prevent="run">
      <h3>Quote calculator</h3>
      <div class="row">
        <label><span>Coupon code</span><input v-model="form.coupon_code" /></label>
        <label><span>Campaign ID</span><input type="number" v-model.number="form.campaign_id" /></label>
        <label><span>Group size</span><input type="number" min="1" v-model.number="form.group_size" /></label>
      </div>
      <h4>Line items</h4>
      <div v-for="(it, i) in form.items" :key="i" class="item">
        <input placeholder="sku" v-model="it.sku" />
        <input placeholder="price" type="number" step="0.01" v-model.number="it.price" />
        <input placeholder="qty"   type="number" min="1" v-model.number="it.quantity" />
        <label class="chk"><input type="checkbox" v-model="it.member_priced" />member-priced</label>
        <button type="button" @click="removeItem(i)" class="x">×</button>
      </div>
      <button type="button" class="ghost" @click="addItem">+ add item</button>
      <button :disabled="busy" type="submit" class="go">{{ busy ? '…' : 'Get quote' }}</button>
      <p v-if="error" class="err">{{ error }}</p>
    </form>

    <section class="panel" v-if="quote">
      <h3>Result</h3>
      <p>Subtotal: <strong>{{ quote.subtotal }}</strong> ({{ quote.currency }})</p>
      <p>Member-priced subtotal: {{ quote.member_priced_subtotal }}</p>
      <p>Eligible subtotal: {{ quote.discount_eligible_subtotal }}</p>
      <p>Discount: <strong>{{ quote.total_discount }}</strong> ({{ quote.discount_percent }}%)</p>
      <p v-if="quote.cap_applied" class="warn">{{ quote.cap_note }}</p>
      <p class="total">Total: {{ quote.total }}</p>

      <h4>Applied</h4>
      <ul>
        <li v-for="(a, i) in quote.applied" :key="i">
          <strong>{{ a.type }}</strong> · {{ a.name || a.code }} · {{ a.kind }} {{ a.value }} → {{ a.amount }}
          <em v-if="a.note"> ({{ a.note }})</em>
        </li>
      </ul>

      <h4 v-if="quote.rejected?.length">Rejected</h4>
      <ul v-if="quote.rejected?.length">
        <li v-for="(r, i) in quote.rejected" :key="i" class="rej">
          {{ r.type }} · {{ r.name || r.code }} — {{ r.reason }}
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.pricing { display: grid; grid-template-columns: minmax(26em, 1fr) 1fr; gap: 1em; align-items: start; }
.panel { background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 1em; }
h3 { margin: 0 0 0.5em; }
h4 { margin: 0.75em 0 0.25em; font-size: 0.9em; color: #6b7280; text-transform: uppercase; }
.row, .item { display: flex; gap: 0.5em; margin-bottom: 0.5em; flex-wrap: wrap; }
label { display: flex; flex-direction: column; gap: 0.2em; font-size: 0.85em; flex: 1; }
input { padding: 0.35em 0.5em; border: 1px solid #ccc; border-radius: 4px; font-size: 0.9em; min-width: 4em; }
.item { align-items: center; }
.chk { flex-direction: row; align-items: center; gap: 0.25em; white-space: nowrap; }
.x { padding: 0 0.5em; border: 1px solid #fecaca; background: #fee2e2; color: #991b1b; border-radius: 4px; cursor: pointer; }
.ghost { padding: 0.3em 0.7em; font-size: 0.85em; background: transparent; border: 1px dashed #9ca3af; color: #374151; cursor: pointer; border-radius: 4px; margin-right: 0.5em; }
.go { padding: 0.4em 1.1em; background: #2563eb; color: white; border: none; border-radius: 4px; cursor: pointer; margin-top: 0.5em; }
.err { color: #b91c1c; margin: 0.5em 0 0; }
.warn { color: #92400e; font-weight: 600; }
.total { font-size: 1.2em; font-weight: 600; }
ul { padding: 0; margin: 0; list-style: none; }
ul li { padding: 0.3em 0; border-bottom: 1px solid #f3f4f6; font-size: 0.9em; }
.rej { color: #6b7280; }
</style>
