<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { UiPage, UiAlert, UiCard, UiInput, UiButton, UiDataTable, UiStatusChip, UiDocumentList, UiRecordDrawer, useConfirm, useListQuery, type Column } from '@go-tangra/ui'
import { zodToFields } from '@go-tangra/ui/forms'
import { LICENSE_LIST, useLicenseList, useInventories } from '@/stores/inventories'
import { useOrg } from '@/stores/org'
import { api, describe } from '@/api/client'
import { licenseSchema, LICENSE_STATUSES } from '@/schemas'
import type { License } from '@/api/types'

const inv = useInventories()
const page = useLicenseList()
const org = useOrg()
const confirm = useConfirm()
const query = ref('')
const dialog = ref(false)
const editing = ref<License | null>(null)
const selected = ref<License | null>(null)
const error = ref('')

// --- server paging and sorting (page / size / sort in the URL: ?licenses.page=…) ---
const lq = useListQuery('licenses', LICENSE_LIST.opts)
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

onMounted(() => {
  void load()
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
    await load()
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
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="reload" />
    </template>
    <template #filters>
      <UiInput id="license-search" v-model="query" label="Search" sr-only-label placeholder="Search licenses" type="search" class="w-full md:max-w-sm" @enter="apply" />
    </template>
    <UiAlert v-if="error || page.error" kind="error" class="mb-3">{{ error || page.error }}</UiAlert>
    <UiCard :padded="false" class="mb-4">
      <UiDataTable :items="page.items" :columns="columns" :loading="page.loading" :total="page.total" :page="lq.page.value" :page-size="lq.pageSize.value" :sort="lq.sort.value" caption="Licenses" empty-title="No licenses" clickable @row-click="selected = $event" @update:page="lq.setPage" @update:page-size="lq.setPageSize" @update:sort="lq.setSort">
        <template #cell-status="{ row }"><UiStatusChip :status="row.status" /></template>
        <template #actions="{ row }">
          <UiButton size="xs" variant="text" icon="mdi-pencil-outline" icon-only label="Edit" @click="edit(row)" />
          <UiButton size="xs" variant="text" icon="mdi-delete-outline" icon-only label="Delete" @click="remove(row)" />
        </template>
      </UiDataTable>
    </UiCard>
    <UiCard v-if="selected"><UiDocumentList :api="api" :base="'licenses/' + selected.id + '/documents'" :title="'Documents — ' + selected.name" /></UiCard>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit license' : 'New license'" :schema="licenseSchema" :fields="fields" :initial="editing ?? { status: 'active' }" :submit="submit" size="lg" @saved="reload" />
  </UiPage>
</template>
