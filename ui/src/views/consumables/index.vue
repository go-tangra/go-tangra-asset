<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useInventories } from '@/stores/inventories'
import { useOrg } from '@/stores/org'
import { describe } from '@/api/client'
import RecordDialog from '@/components/RecordDialog.vue'
import EntityDocuments from '@/components/EntityDocuments.vue'
import type { Field } from '@/components/RecordDialog.vue'
import type { Consumable } from '@/api/types'

const inv = useInventories()
const org = useOrg()
const query = ref('')
const dialog = ref(false)
const editing = ref<Consumable | null>(null)
const selected = ref<Consumable | null>(null)
const error = ref('')

onMounted(() => {
  void inv.loadConsumables()
  void org.loadCategories()
  void org.loadSuppliers()
  void org.loadLocations()
})

const fields = computed<Field[]>(() => [
  { key: 'name', label: 'Name', required: true },
  { key: 'model_name', label: 'Model' },
  { key: 'amount', label: 'Amount in stock', type: 'number' },
  { key: 'min_amount', label: 'Reorder threshold', type: 'number', hint: 'Low stock when amount ≤ threshold' },
  { key: 'category_id', label: 'Category', type: 'select', options: org.categories.map((c) => ({ title: c.name, value: c.id })) },
  { key: 'supplier_id', label: 'Supplier', type: 'select', options: org.suppliers.map((s) => ({ title: s.name, value: s.id })) },
  { key: 'location_id', label: 'Location', type: 'select', options: org.locations.map((l) => ({ title: l.path || l.name, value: l.id })) },
  { key: 'purchase_date', label: 'Purchase date', type: 'date' },
  { key: 'purchase_cost', label: 'Purchase cost', type: 'number' },
  { key: 'order_number', label: 'Order number' },
  { key: 'notes', label: 'Notes', type: 'textarea', cols: 12 },
])

function add(): void {
  editing.value = null
  dialog.value = true
}
function edit(c: Consumable): void {
  editing.value = c
  dialog.value = true
}
async function remove(c: Consumable): Promise<void> {
  error.value = ''
  try {
    await inv.removeConsumable(c.id)
    if (selected.value?.id === c.id) selected.value = null
    await inv.loadConsumables(query.value)
  } catch (e) {
    error.value = describe(e)
  }
}
const submit = (v: Record<string, unknown>) => (editing.value ? inv.updateConsumable(editing.value.id, v) : inv.createConsumable(v))
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Consumables</h1>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-plus" class="me-2" @click="add">New consumable</v-btn>
      <v-btn variant="text" icon="mdi-refresh" @click="inv.loadConsumables(query)" />
    </div>
    <v-text-field v-model="query" label="Search" density="comfortable" clearable class="mb-2" @keyup.enter="inv.loadConsumables(query)" />
    <v-alert v-if="error || inv.error" type="error" variant="tonal" density="compact" class="mb-3">{{ error || inv.error }}</v-alert>
    <v-card class="mb-4">
      <v-table hover>
        <thead><tr><th>Name</th><th>Model</th><th>Category</th><th>Location</th><th class="text-right">Stock</th><th class="text-right">Min</th><th /></tr></thead>
        <tbody>
          <tr v-if="inv.consumables.length === 0"><td colspan="7" class="text-medium-emphasis">No consumables.</td></tr>
          <tr v-for="c in inv.consumables" :key="c.id" style="cursor: pointer" @click="selected = c">
            <td class="font-weight-medium">{{ c.name }} <v-chip v-if="c.low_stock" size="x-small" color="warning" variant="tonal" class="ms-1">low stock</v-chip></td>
            <td>{{ c.model_name || '—' }}</td>
            <td>{{ org.categoryName(c.category_id) || '—' }}</td>
            <td>{{ org.locationName(c.location_id) || '—' }}</td>
            <td class="text-right">{{ c.amount }}</td>
            <td class="text-right">{{ c.min_amount }}</td>
            <td class="text-right">
              <v-btn icon="mdi-pencil-outline" size="small" variant="text" @click.stop="edit(c)" />
              <v-btn icon="mdi-delete-outline" size="small" variant="text" @click.stop="remove(c)" />
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
    <EntityDocuments v-if="selected" entity-type="consumable" :entity-id="selected.id" />
    <RecordDialog v-model="dialog" :title="editing ? 'Edit consumable' : 'New consumable'" :fields="fields" :initial="editing ?? undefined" :submit="submit" @saved="inv.loadConsumables(query)" />
  </div>
</template>
