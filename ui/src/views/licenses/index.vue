<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useInventories } from '@/stores/inventories'
import { useOrg } from '@/stores/org'
import { describe } from '@/api/client'
import RecordDialog from '@/components/RecordDialog.vue'
import EntityDocuments from '@/components/EntityDocuments.vue'
import type { Field } from '@/components/RecordDialog.vue'
import type { License } from '@/api/types'

const inv = useInventories()
const org = useOrg()
const query = ref('')
const dialog = ref(false)
const editing = ref<License | null>(null)
const selected = ref<License | null>(null)
const error = ref('')

onMounted(() => {
  void inv.loadLicenses()
  void org.loadSuppliers()
})

const fields = computed<Field[]>(() => [
  { key: 'name', label: 'Name', required: true },
  { key: 'supplier_id', label: 'Supplier', type: 'select', options: org.suppliers.map((s) => ({ title: s.name, value: s.id })) },
  { key: 'status', label: 'Status', type: 'select', options: ['active', 'expired', 'suspended'].map((s) => ({ title: s, value: s })) },
  { key: 'valid_from', label: 'Valid from', type: 'date' },
  { key: 'valid_to', label: 'Valid to', type: 'date', hint: 'Auto-expires after this date' },
  { key: 'purchase_date', label: 'Purchase date', type: 'date' },
  { key: 'purchase_cost', label: 'Purchase cost', type: 'number' },
  { key: 'order_number', label: 'Order number' },
  { key: 'notes', label: 'Notes', type: 'textarea', cols: 12 },
])
const statusColor: Record<string, string> = { active: 'success', expired: 'error', suspended: 'warning' }

function add(): void {
  editing.value = null
  dialog.value = true
}
function edit(l: License): void {
  editing.value = l
  dialog.value = true
}
async function remove(l: License): Promise<void> {
  error.value = ''
  try {
    await inv.removeLicense(l.id)
    if (selected.value?.id === l.id) selected.value = null
    await inv.loadLicenses(query.value)
  } catch (e) {
    error.value = describe(e)
  }
}
function d(ts?: string): string {
  return ts ? new Date(ts).toLocaleDateString() : '—'
}
const submit = (v: Record<string, unknown>) => (editing.value ? inv.updateLicense(editing.value.id, v) : inv.createLicense(v))
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Licenses</h1>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-plus" class="me-2" @click="add">New license</v-btn>
      <v-btn variant="text" icon="mdi-refresh" @click="inv.loadLicenses(query)" />
    </div>
    <v-text-field v-model="query" label="Search" density="comfortable" clearable class="mb-2" @keyup.enter="inv.loadLicenses(query)" />
    <v-alert v-if="error || inv.error" type="error" variant="tonal" density="compact" class="mb-3">{{ error || inv.error }}</v-alert>
    <v-card class="mb-4">
      <v-table hover>
        <thead><tr><th>Name</th><th>Supplier</th><th>Valid from</th><th>Valid to</th><th>Status</th><th /></tr></thead>
        <tbody>
          <tr v-if="inv.licenses.length === 0"><td colspan="6" class="text-medium-emphasis">No licenses.</td></tr>
          <tr v-for="l in inv.licenses" :key="l.id" style="cursor: pointer" @click="selected = l">
            <td class="font-weight-medium">{{ l.name }}</td>
            <td>{{ org.supplierName(l.supplier_id) || '—' }}</td>
            <td>{{ d(l.valid_from) }}</td>
            <td>{{ d(l.valid_to) }}</td>
            <td><v-chip size="small" variant="tonal" :color="statusColor[l.status]">{{ l.status }}</v-chip></td>
            <td class="text-right">
              <v-btn icon="mdi-pencil-outline" size="small" variant="text" @click.stop="edit(l)" />
              <v-btn icon="mdi-delete-outline" size="small" variant="text" @click.stop="remove(l)" />
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
    <EntityDocuments v-if="selected" entity-type="license" :entity-id="selected.id" />
    <RecordDialog v-model="dialog" :title="editing ? 'Edit license' : 'New license'" :fields="fields" :initial="editing ?? { status: 'active' }" :submit="submit" @saved="inv.loadLicenses(query)" />
  </div>
</template>
