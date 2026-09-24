<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { UiPage, UiAlert, UiCard, UiInput, UiButton, UiDataTable, UiStatusChip, UiRecordDrawer, useConfirm, type Column } from '@go-tangra/ui'
import { zodToFields } from '@go-tangra/ui/forms'
import { useOrg } from '@/stores/org'
import { describe } from '@/api/client'
import { supplierSchema, SUPPLIER_STATUSES } from '@/schemas'
import type { Supplier } from '@/api/types'

const org = useOrg()
const confirm = useConfirm()
const query = ref('')
const dialog = ref(false)
const editing = ref<Supplier | null>(null)
const error = ref('')

onMounted(() => void org.loadSuppliers())

const fields = zodToFields(supplierSchema, {
  status: { type: 'select', options: SUPPLIER_STATUSES.map((s) => ({ title: s, value: s })) },
  contact_person: { label: 'Contact person (sealed)' },
  telephone: { label: 'Telephone (sealed)' },
  email: { label: 'E-mail (sealed)' },
})
const columns: Column<Supplier>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'code', label: 'Code', hideOnStack: true },
  { key: 'city', label: 'City' },
  { key: 'country', label: 'Country', hideOnStack: true },
  { key: 'contact', label: 'Contact', format: (s) => s.contact_person || s.email || '(redacted)' },
  { key: 'status', label: 'Status', width: 'sm' },
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
  if (!(await confirm.ask({ title: `Delete ${s.name}?`, text: 'Assets keep their history; the supplier record is removed.', danger: true, confirmLabel: 'Delete' }))) return
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
  <UiPage title="Suppliers">
    <template #actions>
      <UiButton icon="mdi-plus" @click="add">New supplier</UiButton>
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="org.loadSuppliers(query)" />
    </template>
    <template #filters>
      <UiInput id="supplier-search" v-model="query" label="Search" sr-only-label placeholder="Search suppliers" type="search" class="w-full md:max-w-sm" @enter="org.loadSuppliers(query)" />
    </template>
    <UiAlert v-if="error || org.error" kind="error" class="mb-3">{{ error || org.error }}</UiAlert>
    <UiCard :padded="false">
      <UiDataTable :items="org.suppliers" :columns="columns" caption="Suppliers" empty-title="No suppliers">
        <template #cell-status="{ row }"><UiStatusChip :status="row.status" /></template>
        <template #actions="{ row }">
          <UiButton size="xs" variant="text" icon="mdi-pencil-outline" icon-only label="Edit" @click="edit(row)" />
          <UiButton size="xs" variant="text" icon="mdi-delete-outline" icon-only label="Delete" @click="remove(row)" />
        </template>
      </UiDataTable>
    </UiCard>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit supplier' : 'New supplier'" :schema="supplierSchema" :fields="fields" :initial="editing ?? { status: 'active' }" :submit="submit" size="lg" @saved="org.loadSuppliers(query)" />
  </UiPage>
</template>
