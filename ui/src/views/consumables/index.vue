<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { UiPage, UiAlert, UiCard, UiInput, UiButton, UiDataTable, UiBadge, UiDocumentList, UiRecordDrawer, useConfirm, useListQuery, type Column } from '@go-tangra/ui'
import { zodToFields } from '@go-tangra/ui/forms'
import { CONSUMABLE_LIST, useConsumableList, useInventories } from '@/stores/inventories'
import { useOrg } from '@/stores/org'
import { api, describe } from '@/api/client'
import { consumableSchema } from '@/schemas'
import type { Consumable } from '@/api/types'

const inv = useInventories()
const page = useConsumableList()
const org = useOrg()
const confirm = useConfirm()
const query = ref('')
const dialog = ref(false)
const editing = ref<Consumable | null>(null)
const selected = ref<Consumable | null>(null)
const error = ref('')

// --- server paging and sorting (page / size / sort in the URL: ?consumables.page=…) ---
const lq = useListQuery('consumables', CONSUMABLE_LIST.opts)
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
  void org.loadCategories()
  void org.loadSuppliers()
  void org.loadLocations()
})

const fields = computed(() =>
  zodToFields(consumableSchema, {
    model_name: { label: 'Model' },
    amount: { label: 'Amount in stock' },
    min_amount: { label: 'Reorder threshold', hint: 'Low stock when amount ≤ threshold' },
    category_id: { type: 'select', options: org.categories.map((c) => ({ title: c.name, value: c.id })) },
    supplier_id: { type: 'select', options: org.suppliers.map((s) => ({ title: s.name, value: s.id })) },
    location_id: { type: 'select', options: org.locations.map((l) => ({ title: l.path || l.name, value: l.id })) },
  }),
)
const columns: Column<Consumable>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'model_name', label: 'Model', hideOnStack: true },
  { key: 'category_id', label: 'Category', format: (c) => org.categoryName(c.category_id) ?? '' },
  { key: 'location_id', label: 'Location', format: (c) => org.locationName(c.location_id) ?? '', hideOnStack: true },
  { key: 'amount', label: 'Stock', align: 'end', sortable: true },
  { key: 'min_amount', label: 'Min', align: 'end' },
]

function add(): void {
  editing.value = null
  dialog.value = true
}
function edit(c: Consumable): void {
  editing.value = c
  dialog.value = true
}
async function remove(c: Consumable): Promise<void> {
  if (!(await confirm.ask({ title: `Delete ${c.name}?`, danger: true, confirmLabel: 'Delete' }))) return
  error.value = ''
  try {
    await inv.removeConsumable(c.id)
    if (selected.value?.id === c.id) selected.value = null
    await load()
  } catch (e) {
    error.value = describe(e)
  }
}
const submit = (v: Record<string, unknown>) => (editing.value ? inv.updateConsumable(editing.value.id, v) : inv.createConsumable(v))
</script>

<template>
  <UiPage title="Consumables">
    <template #actions>
      <UiButton icon="mdi-plus" @click="add">New consumable</UiButton>
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="reload" />
    </template>
    <template #filters>
      <UiInput id="consumable-search" v-model="query" label="Search" sr-only-label placeholder="Search consumables" type="search" class="w-full md:max-w-sm" @enter="apply" />
    </template>
    <UiAlert v-if="error || page.error" kind="error" class="mb-3">{{ error || page.error }}</UiAlert>
    <UiCard :padded="false" class="mb-4">
      <UiDataTable :items="page.items" :columns="columns" :loading="page.loading" :total="page.total" :page="lq.page.value" :page-size="lq.pageSize.value" :sort="lq.sort.value" caption="Consumables" empty-title="No consumables" clickable @row-click="selected = $event" @update:page="lq.setPage" @update:page-size="lq.setPageSize" @update:sort="lq.setSort">
        <template #cell-name="{ row }">{{ row.name }} <UiBadge v-if="row.low_stock" color="warning" size="xs">low stock</UiBadge></template>
        <template #actions="{ row }">
          <UiButton size="xs" variant="text" icon="mdi-pencil-outline" icon-only label="Edit" @click="edit(row)" />
          <UiButton size="xs" variant="text" icon="mdi-delete-outline" icon-only label="Delete" @click="remove(row)" />
        </template>
      </UiDataTable>
    </UiCard>
    <UiCard v-if="selected"><UiDocumentList :api="api" :base="'consumables/' + selected.id + '/documents'" :title="'Documents — ' + selected.name" /></UiCard>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit consumable' : 'New consumable'" :schema="consumableSchema" :fields="fields" :initial="editing ?? undefined" :submit="submit" size="lg" @saved="reload" />
  </UiPage>
</template>
