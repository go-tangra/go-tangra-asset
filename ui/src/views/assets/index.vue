<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { UiPage, UiAlert, UiCard, UiInput, UiSelect, UiButton, UiDataTable, UiStatusChip, UiLiveIndicator, UiRecordDrawer, useListQuery, type Column, type SelectOption } from '@go-tangra/ui'
import { zodToFields } from '@go-tangra/ui/forms'
import { ASSET_LIST, useAssets, type AssetFilter } from '@/stores/assets'
import { useOrg } from '@/stores/org'
import { coalesce, useLive } from '@/stores/live'
import { assetSchema, ASSET_STATUSES, type AssetInput } from '@/schemas'
import type { Asset } from '@/api/types'

const router = useRouter()
const store = useAssets()
const org = useOrg()
const live = useLive()

const query = ref('')
const status = ref<string | undefined>()
const category = ref<string | undefined>()
const dialog = ref(false)

const statusOptions: SelectOption[] = ['deployable', 'assigned', 'broken', 'archived'].map((s) => ({ title: s, value: s }))
const categoryOptions = computed<SelectOption[]>(() => org.categories.map((c) => ({ title: c.name, value: c.id })))

// --- server paging and sorting (page / size / sort in the URL: ?assets.page=…) ---
const lq = useListQuery('assets', ASSET_LIST.opts)
const filterValue = (): AssetFilter => ({ query: query.value.trim() || undefined, status: status.value || undefined, category_id: category.value || undefined })
async function load(): Promise<void> {
  const res = await store.list(filterValue(), lq.query.value)
  if (res?.page) lq.clampTo(res.page) // a page beyond the end answers the last page
}
watch(lq.query, () => void load())
/** Filters changed: back to page 1 (which reloads), or reload in place. */
function apply(): void {
  if (lq.page.value !== 1) lq.resetPage()
  else void load()
}
/** Reloads the current page (after a create, on Refresh or a live event). */
const reload = () => void load()

// Any asset.* event may change the visible rows: one reload per burst.
const refresh = coalesce(reload)
let release: (() => void) | null = null
let off: (() => void) | null = null
onMounted(() => {
  void load()
  void org.loadCategories()
  void org.loadSuppliers()
  void org.loadLocations()
  release = live.connect()
  off = live.on((type) => {
    if (type.startsWith('asset.')) refresh.trigger()
  })
})
onUnmounted(() => {
  release?.()
  off?.()
  refresh.cancel()
})

const fields = computed(() =>
  zodToFields(assetSchema, {
    asset_tag: { hint: 'Blank → generated (AST-xxxxxx)' },
    model_name: { label: 'Model' },
    category_id: { type: 'select', options: categoryOptions.value },
    supplier_id: { type: 'select', options: org.suppliers.map((s) => ({ title: s.name, value: s.id })) },
    location_id: { type: 'select', options: org.locations.map((l) => ({ title: l.path || l.name, value: l.id })) },
    status: { type: 'select', options: ASSET_STATUSES.map((s) => ({ title: s, value: s })) },
    warranty_months: { label: 'Warranty (months)' },
    useful_life_years: { label: 'Useful life (years)' },
    depreciation_rate: { label: 'Depreciation rate (0–1)', hint: 'Blank/0 → 0.40' },
  }),
)
// Sortable columns are the server's sort fields (ASSET_LIST): sorting orders
// the whole list, not the visible page. Category and location sort by the
// referenced record's name / path.
const columns: Column<Asset>[] = [
  { key: 'asset_tag', label: 'Tag', sortable: true, width: 'sm' },
  { key: 'name', label: 'Name', sortable: true },
  { key: 'serial', label: 'Serial', hideOnStack: true },
  { key: 'category', label: 'Category', sortable: true, format: (a) => org.categoryName(a.category_id) ?? '' },
  { key: 'status', label: 'Status', width: 'sm', sortable: true },
  { key: 'assignee_name', label: 'Assignee', format: (a) => a.assignee_name || a.user_id || '' },
  { key: 'location', label: 'Location', sortable: true, format: (a) => org.locationName(a.location_id) ?? '', hideOnStack: true },
  { key: 'purchase_date', label: 'Purchased', sortable: true, defaultDir: 'desc', format: (a) => (a.purchase_date ? new Date(a.purchase_date).toLocaleDateString() : ''), hideOnStack: true },
  { key: 'book_value', label: 'Book value', align: 'end', format: (a) => money(a.book_value) },
]

function open(a: Asset): void {
  void router.push({ name: 'asset-detail', params: { id: a.id } })
}
function money(v?: number): string {
  return v === undefined ? '' : v.toLocaleString(undefined, { maximumFractionDigits: 2 })
}
</script>

<template>
  <UiPage title="Assets">
    <template #badges><UiLiveIndicator :connected="live.connected" /></template>
    <template #actions>
      <UiButton icon="mdi-plus" @click="dialog = true">New asset</UiButton>
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="reload" />
    </template>
    <template #filters>
      <div class="grid w-full grid-cols-2 gap-2 md:grid-cols-12 md:items-end">
        <div class="col-span-2 md:col-span-5"><UiInput id="asset-search" v-model="query" label="Search (name, tag, serial, model)" type="search" size="sm" @enter="apply" /></div>
        <div class="md:col-span-3"><UiSelect id="asset-status" v-model="status" label="Status" :options="statusOptions" size="sm" @update:model-value="apply" /></div>
        <div class="md:col-span-3"><UiSelect id="asset-category" v-model="category" label="Category" :options="categoryOptions" size="sm" @update:model-value="apply" /></div>
        <div class="col-span-2 md:col-span-1"><UiButton block variant="soft" size="sm" @click="apply">Filter</UiButton></div>
      </div>
    </template>
    <UiAlert v-if="store.error" kind="error" class="mb-3">{{ store.error }}</UiAlert>
    <UiCard :padded="false">
      <UiDataTable :items="store.items" :columns="columns" :loading="store.loading" :total="store.total" :page="lq.page.value" :page-size="lq.pageSize.value" :sort="lq.sort.value" caption="Assets" empty-title="No assets" clickable @row-click="open" @update:page="lq.setPage" @update:page-size="lq.setPageSize" @update:sort="lq.setSort">
        <template #cell-status="{ row }"><UiStatusChip :status="row.status" /></template>
      </UiDataTable>
    </UiCard>
    <UiRecordDrawer v-model="dialog" close-on-save title="New asset" :schema="assetSchema" :fields="fields" :submit="(v) => store.create(v as AssetInput)" size="xl" @saved="reload" />
  </UiPage>
</template>
