<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAssets } from '@/stores/assets'
import { useOrg } from '@/stores/org'
import { useLive } from '@/stores/live'
import RecordDialog from '@/components/RecordDialog.vue'
import type { Field } from '@/components/RecordDialog.vue'
import type { AssetInput } from '@/api/types'

const router = useRouter()
const store = useAssets()
const org = useOrg()
const live = useLive()

const query = ref('')
const status = ref<string | null>(null)
const category = ref<string | null>(null)
const dialog = ref(false)

const STATUSES = ['deployable', 'assigned', 'broken', 'archived']
const statusColor: Record<string, string> = { deployable: 'success', assigned: 'info', broken: 'error', archived: 'grey' }

let release: (() => void) | null = null
let off: (() => void) | null = null
onMounted(() => {
  void store.list()
  void org.loadCategories()
  void org.loadSuppliers()
  void org.loadLocations()
  release = live.connect()
  off = live.on((type) => {
    if (type === 'asset.assigned' || type === 'asset.unassigned') reload()
  })
})
onUnmounted(() => {
  release?.()
  off?.()
})

function reload(): void {
  void store.list({ query: query.value.trim() || undefined, status: status.value ?? undefined, category_id: category.value ?? undefined })
}

const fields = computed<Field[]>(() => [
  { key: 'name', label: 'Name', required: true },
  { key: 'asset_tag', label: 'Asset tag', hint: 'Blank → generated (AST-xxxxxx)' },
  { key: 'serial', label: 'Serial' },
  { key: 'model_name', label: 'Model' },
  { key: 'category_id', label: 'Category', type: 'select', options: org.categories.map((c) => ({ title: c.name, value: c.id })) },
  { key: 'supplier_id', label: 'Supplier', type: 'select', options: org.suppliers.map((s) => ({ title: s.name, value: s.id })) },
  { key: 'location_id', label: 'Location', type: 'select', options: org.locations.map((l) => ({ title: l.path || l.name, value: l.id })) },
  { key: 'status', label: 'Status', type: 'select', options: ['deployable', 'broken', 'archived'].map((s) => ({ title: s, value: s })) },
  { key: 'purchase_date', label: 'Purchase date', type: 'date' },
  { key: 'purchase_cost', label: 'Purchase cost', type: 'number' },
  { key: 'warranty_months', label: 'Warranty (months)', type: 'number' },
  { key: 'useful_life_years', label: 'Useful life (years)', type: 'number' },
  { key: 'salvage_value', label: 'Salvage value', type: 'number' },
  { key: 'depreciation_rate', label: 'Depreciation rate (0–1)', type: 'number', hint: 'Blank/0 → 0.40' },
  { key: 'notes', label: 'Notes', type: 'textarea', cols: 12 },
])

function open(id: string): void {
  void router.push({ name: 'asset-detail', params: { id } })
}

function money(v?: number): string {
  return v === undefined ? '—' : v.toLocaleString(undefined, { maximumFractionDigits: 2 })
}
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Assets</h1>
      <v-chip v-if="live.connected" size="x-small" color="success" variant="tonal" class="ms-3">live</v-chip>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-plus" class="me-2" @click="dialog = true">New asset</v-btn>
      <v-btn variant="text" icon="mdi-refresh" @click="reload" />
    </div>
    <v-row dense class="mb-2">
      <v-col cols="12" md="5"><v-text-field v-model="query" label="Search (name, tag, serial, model)" density="comfortable" clearable @keyup.enter="reload" /></v-col>
      <v-col cols="6" md="3"><v-select v-model="status" :items="STATUSES" label="Status" density="comfortable" clearable @update:model-value="reload" /></v-col>
      <v-col cols="6" md="3"><v-select v-model="category" :items="org.categories.map((c) => ({ title: c.name, value: c.id }))" label="Category" density="comfortable" clearable @update:model-value="reload" /></v-col>
      <v-col cols="12" md="1"><v-btn block variant="tonal" @click="reload">Filter</v-btn></v-col>
    </v-row>
    <v-alert v-if="store.error" type="error" variant="tonal" density="compact" class="mb-3">{{ store.error }}</v-alert>
    <v-card>
      <v-table hover>
        <thead>
          <tr><th>Tag</th><th>Name</th><th>Serial</th><th>Category</th><th>Status</th><th>Assignee</th><th>Location</th><th class="text-right">Book value</th></tr>
        </thead>
        <tbody>
          <tr v-if="!store.loading && store.items.length === 0"><td colspan="8" class="text-medium-emphasis">No assets.</td></tr>
          <tr v-for="a in store.items" :key="a.id" style="cursor: pointer" @click="open(a.id)">
            <td class="font-weight-medium">{{ a.asset_tag }}</td>
            <td>{{ a.name }}</td>
            <td>{{ a.serial || '—' }}</td>
            <td>{{ org.categoryName(a.category_id) || '—' }}</td>
            <td><v-chip size="small" :color="statusColor[a.status]" variant="tonal">{{ a.status }}</v-chip></td>
            <td>{{ a.assignee_name || (a.user_id ? a.user_id : '—') }}</td>
            <td>{{ org.locationName(a.location_id) || '—' }}</td>
            <td class="text-right">{{ money(a.book_value) }}</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
    <RecordDialog v-model="dialog" title="New asset" :fields="fields" :submit="(v) => store.create(v as unknown as AssetInput)" @saved="reload" />
  </div>
</template>
