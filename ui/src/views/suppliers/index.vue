<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useOrg } from '@/stores/org'
import { describe } from '@/api/client'
import RecordDialog from '@/components/RecordDialog.vue'
import type { Field } from '@/components/RecordDialog.vue'
import type { Supplier } from '@/api/types'

const org = useOrg()
const query = ref('')
const dialog = ref(false)
const editing = ref<Supplier | null>(null)
const error = ref('')

onMounted(() => void org.loadSuppliers())

const fields: Field[] = [
  { key: 'name', label: 'Name', required: true },
  { key: 'code', label: 'Code' },
  { key: 'status', label: 'Status', type: 'select', options: ['active', 'inactive'].map((s) => ({ title: s, value: s })) },
  { key: 'website', label: 'Website' },
  { key: 'address', label: 'Address' },
  { key: 'city', label: 'City' },
  { key: 'state', label: 'State' },
  { key: 'country', label: 'Country' },
  { key: 'postal_code', label: 'Postal code' },
  { key: 'contact_person', label: 'Contact person (sealed)' },
  { key: 'telephone', label: 'Telephone (sealed)' },
  { key: 'email', label: 'E-mail (sealed)' },
  { key: 'notes', label: 'Notes', type: 'textarea', cols: 12 },
]

function add(): void {
  editing.value = null
  dialog.value = true
}
function edit(s: Supplier): void {
  editing.value = s
  dialog.value = true
}
async function remove(s: Supplier): Promise<void> {
  error.value = ''
  try {
    await org.removeSupplier(s.id)
    await org.loadSuppliers(query.value)
  } catch (e) {
    error.value = describe(e)
  }
}
const submit = (v: Record<string, unknown>) => (editing.value ? org.updateSupplier(editing.value.id, v) : org.createSupplier(v))
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Suppliers</h1>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-plus" class="me-2" @click="add">New supplier</v-btn>
      <v-btn variant="text" icon="mdi-refresh" @click="org.loadSuppliers(query)" />
    </div>
    <v-text-field v-model="query" label="Search" density="comfortable" clearable class="mb-2" @keyup.enter="org.loadSuppliers(query)" />
    <v-alert v-if="error || org.error" type="error" variant="tonal" density="compact" class="mb-3">{{ error || org.error }}</v-alert>
    <v-card>
      <v-table hover>
        <thead><tr><th>Name</th><th>Code</th><th>City</th><th>Country</th><th>Contact</th><th>Status</th><th /></tr></thead>
        <tbody>
          <tr v-if="org.suppliers.length === 0"><td colspan="7" class="text-medium-emphasis">No suppliers.</td></tr>
          <tr v-for="s in org.suppliers" :key="s.id">
            <td class="font-weight-medium">{{ s.name }}</td>
            <td>{{ s.code || '—' }}</td>
            <td>{{ s.city || '—' }}</td>
            <td>{{ s.country || '—' }}</td>
            <td>{{ s.contact_person || s.email || '(redacted)' }}</td>
            <td><v-chip size="small" variant="tonal" :color="s.status === 'active' ? 'success' : 'grey'">{{ s.status }}</v-chip></td>
            <td class="text-right">
              <v-btn icon="mdi-pencil-outline" size="small" variant="text" @click="edit(s)" />
              <v-btn icon="mdi-delete-outline" size="small" variant="text" @click="remove(s)" />
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
    <RecordDialog v-model="dialog" :title="editing ? 'Edit supplier' : 'New supplier'" :fields="fields" :initial="editing ?? { status: 'active' }" :submit="submit" @saved="org.loadSuppliers(query)" />
  </div>
</template>
