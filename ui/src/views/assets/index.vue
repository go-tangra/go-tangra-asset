<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiPage, UiAlert, UiCard, UiInput, UiSelect, UiButton, UiDataTable, UiStatusChip, UiLiveIndicator, UiRecordDrawer, type Column, type SelectOption } from '@freya/ui'
import { zodToFields } from '@freya/ui/forms'
import { useAssets } from '@/stores/assets'
import { useOrg } from '@/stores/org'
import { useLive } from '@/stores/live'
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
  void store.list({ query: query.value.trim() || undefined, status: status.value || undefined, category_id: category.value || undefined })
}

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
const columns: Column<Asset>[] = [
  { key: 'asset_tag', label: 'Tag', sortable: true, width: 'sm' },
  { key: 'name', label: 'Name', sortable: true },
  { key: 'serial', label: 'Serial', hideOnStack: true },
  { key: 'category_id', label: 'Category', format: (a) => org.categoryName(a.category_id) ?? '' },
  { key: 'status', label: 'Status', width: 'sm' },
  { key: 'assignee_name', label: 'Assignee', format: (a) => a.assignee_name || a.user_id || '' },
  { key: 'location_id', label: 'Location', format: (a) => org.locationName(a.location_id) ?? '', hideOnStack: true },
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
        <div class="col-span-2 md:col-span-5"><UiInput id="asset-search" v-model="query" label="Search (name, tag, serial, model)" type="search" size="sm" @enter="reload" /></div>
        <div class="md:col-span-3"><UiSelect id="asset-status" v-model="status" label="Status" :options="statusOptions" size="sm" @update:model-value="reload" /></div>
        <div class="md:col-span-3"><UiSelect id="asset-category" v-model="category" label="Category" :options="categoryOptions" size="sm" @update:model-value="reload" /></div>
        <div class="col-span-2 md:col-span-1"><UiButton block variant="soft" size="sm" @click="reload">Filter</UiButton></div>
      </div>
    </template>
    <UiAlert v-if="store.error" kind="error" class="mb-3">{{ store.error }}</UiAlert>
    <UiCard :padded="false">
      <UiDataTable :items="store.items" :columns="columns" :loading="store.loading" caption="Assets" empty-title="No assets" clickable @row-click="open">
        <template #cell-status="{ row }"><UiStatusChip :status="row.status" /></template>
      </UiDataTable>
    </UiCard>
    <UiRecordDrawer v-model="dialog" close-on-save title="New asset" :schema="assetSchema" :fields="fields" :submit="(v) => store.create(v as AssetInput)" size="xl" @saved="reload" />
  </UiPage>
</template>
