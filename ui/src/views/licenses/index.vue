<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { UiPage, UiAlert, UiCard, UiInput, UiButton, UiDataTable, UiStatusChip, UiDocumentList, UiRecordDrawer, useConfirm, type Column } from '@freya/ui'
import { zodToFields } from '@freya/ui/forms'
import { useInventories } from '@/stores/inventories'
import { useOrg } from '@/stores/org'
import { api, describe } from '@/api/client'
import { licenseSchema, LICENSE_STATUSES } from '@/schemas'
import type { License } from '@/api/types'

const inv = useInventories()
const org = useOrg()
const confirm = useConfirm()
const query = ref('')
const dialog = ref(false)
const editing = ref<License | null>(null)
const selected = ref<License | null>(null)
const error = ref('')

onMounted(() => {
  void inv.loadLicenses()
  void org.loadSuppliers()
})

const fields = computed(() =>
  zodToFields(licenseSchema, {
    supplier_id: { type: 'select', options: org.suppliers.map((s) => ({ title: s.name, value: s.id })) },
    status: { type: 'select', options: LICENSE_STATUSES.map((s) => ({ title: s, value: s })) },
    valid_to: { hint: 'Auto-expires after this date' },
  }),
)
const d = (ts?: string): string => (ts ? new Date(ts).toLocaleDateString() : '')
const columns: Column<License>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'supplier_id', label: 'Supplier', format: (l) => org.supplierName(l.supplier_id) ?? '' },
  { key: 'valid_from', label: 'Valid from', format: (l) => d(l.valid_from), hideOnStack: true },
  { key: 'valid_to', label: 'Valid to', format: (l) => d(l.valid_to), sortable: true },
  { key: 'status', label: 'Status', width: 'sm' },
]

function add(): void {
  editing.value = null
  dialog.value = true
}
function edit(l: License): void {
  editing.value = l
  dialog.value = true
}
async function remove(l: License): Promise<void> {
  if (!(await confirm.ask({ title: `Delete ${l.name}?`, danger: true, confirmLabel: 'Delete' }))) return
  error.value = ''
  try {
    await inv.removeLicense(l.id)
    if (selected.value?.id === l.id) selected.value = null
    await inv.loadLicenses(query.value)
  } catch (e) {
    error.value = describe(e)
  }
}
const submit = (v: Record<string, unknown>) => (editing.value ? inv.updateLicense(editing.value.id, v) : inv.createLicense(v))
</script>

<template>
  <UiPage title="Licenses">
    <template #actions>
      <UiButton icon="mdi-plus" @click="add">New license</UiButton>
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="inv.loadLicenses(query)" />
    </template>
    <template #filters>
      <UiInput id="license-search" v-model="query" label="Search" sr-only-label placeholder="Search licenses" type="search" class="w-full md:max-w-sm" @enter="inv.loadLicenses(query)" />
    </template>
    <UiAlert v-if="error || inv.error" kind="error" class="mb-3">{{ error || inv.error }}</UiAlert>
    <UiCard :padded="false" class="mb-4">
      <UiDataTable :items="inv.licenses" :columns="columns" caption="Licenses" empty-title="No licenses" clickable @row-click="selected = $event">
        <template #cell-status="{ row }"><UiStatusChip :status="row.status" /></template>
        <template #actions="{ row }">
          <UiButton size="xs" variant="text" icon="mdi-pencil-outline" icon-only label="Edit" @click="edit(row)" />
          <UiButton size="xs" variant="text" icon="mdi-delete-outline" icon-only label="Delete" @click="remove(row)" />
        </template>
      </UiDataTable>
    </UiCard>
    <UiCard v-if="selected"><UiDocumentList :api="api" :base="'licenses/' + selected.id + '/documents'" :title="'Documents — ' + selected.name" /></UiCard>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit license' : 'New license'" :schema="licenseSchema" :fields="fields" :initial="editing ?? { status: 'active' }" :submit="submit" size="lg" @saved="inv.loadLicenses(query)" />
  </UiPage>
</template>
