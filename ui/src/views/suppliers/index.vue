<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { UiPage, UiAlert, UiCard, UiInput, UiButton, UiDataTable, UiStatusChip, UiRecordDrawer, useConfirm, useListQuery, type Column } from '@go-tangra/ui'
import { zodToFields } from '@go-tangra/ui/forms'
import { SUPPLIER_LIST, useOrg, useSupplierList } from '@/stores/org'
import { describe } from '@/api/client'
import { supplierSchema, SUPPLIER_STATUSES } from '@/schemas'
import type { Supplier } from '@/api/types'

const org = useOrg()
const page = useSupplierList()
const confirm = useConfirm()
const query = ref('')
const dialog = ref(false)
const editing = ref<Supplier | null>(null)
const error = ref('')

// --- server paging and sorting (page / size / sort in the URL: ?suppliers.page=…) ---
const lq = useListQuery('suppliers', SUPPLIER_LIST.opts)
async function load(): Promise<void> {
  const res = await page.list({ query: query.value.trim() || undefined }, lq.query.value)
  if (res?.page) lq.clampTo(res.page) // a page beyond the end answers the last page
}
watch(lq.query, () => void load())
/** Search changed: back to page 1 (which reloads), or reload in place. */
function apply(): void {
  if (lq.page.value !== 1) lq.resetPage()
  else void load()
}
const reload = () => void load()
onMounted(reload)

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
    await load()
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
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="reload" />
    </template>
    <template #filters>
      <UiInput id="supplier-search" v-model="query" label="Search" sr-only-label placeholder="Search suppliers" type="search" class="w-full md:max-w-sm" @enter="apply" />
    </template>
    <UiAlert v-if="error || page.error" kind="error" class="mb-3">{{ error || page.error }}</UiAlert>
    <UiCard :padded="false">
      <UiDataTable :items="page.items" :columns="columns" :loading="page.loading" :total="page.total" :page="lq.page.value" :page-size="lq.pageSize.value" :sort="lq.sort.value" caption="Suppliers" empty-title="No suppliers" @update:page="lq.setPage" @update:page-size="lq.setPageSize" @update:sort="lq.setSort">
        <template #cell-status="{ row }"><UiStatusChip :status="row.status" /></template>
        <template #actions="{ row }">
          <UiButton size="xs" variant="text" icon="mdi-pencil-outline" icon-only label="Edit" @click="edit(row)" />
          <UiButton size="xs" variant="text" icon="mdi-delete-outline" icon-only label="Delete" @click="remove(row)" />
        </template>
      </UiDataTable>
    </UiCard>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit supplier' : 'New supplier'" :schema="supplierSchema" :fields="fields" :initial="editing ?? { status: 'active' }" :submit="submit" size="lg" @saved="reload" />
  </UiPage>
</template>
