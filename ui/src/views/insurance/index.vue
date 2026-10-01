<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { UiPage, UiAlert, UiCard, UiInput, UiButton, UiDataTable, UiStatusChip, UiForm, UiCombobox, UiNumberInput, UiRecordDrawer, useConfirm, useListQuery, type Column } from '@go-tangra/ui'
import { zodToFields, useZodForm } from '@go-tangra/ui/forms'
import { COVERED_LIST, POLICY_LIST, useInventories, usePolicyList } from '@/stores/inventories'
import { pagedList } from '@/stores/paged'
import { useAssets } from '@/stores/assets'
import { describe } from '@/api/client'
import { insurancePolicySchema, policyAssetSchema, POLICY_STATUSES, COVERAGE_TYPES } from '@/schemas'
import type { InsurancePolicy, PolicyAsset } from '@/api/types'

const inv = useInventories()
const page = usePolicyList()
const assets = useAssets()
const confirm = useConfirm()
const query = ref('')
const dialog = ref(false)
const editing = ref<InsurancePolicy | null>(null)
const selected = ref<InsurancePolicy | null>(null)
const error = ref('')

// --- server paging and sorting (page / size / sort in the URL: ?policies.page=…) ---
const lq = useListQuery('policies', POLICY_LIST.opts)
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

// --- the selected policy's covered assets: a server-paged table too (?covered.page=…) ---
const cq = useListQuery('covered', COVERED_LIST.opts)
const covered = reactive(pagedList<PolicyAsset>(() => 'insurance-policies/' + (selected.value?.id ?? '') + '/assets', COVERED_LIST.first))
async function loadCovered(): Promise<void> {
  if (!selected.value) return
  const res = await covered.list({}, cq.query.value)
  if (res?.page) cq.clampTo(res.page)
}
watch(cq.query, () => void loadCovered())

onMounted(() => {
  void load()
  void assets.searchOptions()
})
const assetOptions = computed(() => assets.options.map((a) => ({ title: a.asset_tag + ' · ' + (a.name || ''), value: a.id })))

const fields = zodToFields(insurancePolicySchema, {
  coverage_type: { type: 'select', options: COVERAGE_TYPES.map((s) => ({ title: s, value: s })) },
  status: { type: 'select', options: POLICY_STATUSES.map((s) => ({ title: s, value: s })) },
  premium_amount: { label: 'Premium' },
})
const d = (ts?: string): string => (ts ? new Date(ts).toLocaleDateString() : '')
const columns: Column<InsurancePolicy>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'policy_number', label: 'Number', hideOnStack: true },
  { key: 'provider', label: 'Provider', hideOnStack: true },
  { key: 'valid_to', label: 'Valid to', format: (p) => d(p.valid_to), sortable: true },
  { key: 'status', label: 'Status', width: 'sm' },
  { key: 'asset_count', label: 'Assets', align: 'end' },
]
const coveredColumns: Column<PolicyAsset>[] = [
  { key: 'asset_tag', label: 'Tag', sortable: true },
  { key: 'name', label: 'Name', sortable: true, format: (pa) => pa.asset_name ?? '' },
  { key: 'covered_value', label: 'Covered', align: 'end', format: (pa) => String(pa.covered_value ?? 0) },
]

/** Selecting another policy shows its first covered page. */
function select(p: InsurancePolicy): void {
  const other = selected.value?.id !== p.id
  selected.value = p
  if (other && cq.page.value !== 1) cq.resetPage()
  else void loadCovered()
}
function add(): void {
  editing.value = null
  dialog.value = true
}
function edit(p: InsurancePolicy): void {
  editing.value = p
  dialog.value = true
}
async function remove(p: InsurancePolicy): Promise<void> {
  if (!(await confirm.ask({ title: `Delete ${p.name}?`, danger: true, confirmLabel: 'Delete' }))) return
  error.value = ''
  try {
    await inv.removePolicy(p.id)
    if (selected.value?.id === p.id) selected.value = null
    await load()
  } catch (e) {
    error.value = describe(e)
  }
}
const coverForm = useZodForm(policyAssetSchema, {
  initial: { asset_id: '', covered_value: 0 },
  onSubmit: (v) => inv.addPolicyAsset(selected.value!.id, v.asset_id, v.covered_value),
  onSuccess: async () => {
    coverForm.reset({ asset_id: '', covered_value: 0 })
    await loadCovered()
    await load()
  },
})
async function uncover(pa: PolicyAsset): Promise<void> {
  if (!selected.value) return
  try {
    await inv.removePolicyAsset(selected.value.id, pa.asset_id)
    await loadCovered()
    await load()
  } catch (e) {
    error.value = describe(e)
  }
}
const submit = (v: Record<string, unknown>) => (editing.value ? inv.updatePolicy(editing.value.id, v) : inv.createPolicy(v))
</script>

<template>
  <UiPage title="Insurance policies">
    <template #actions>
      <UiButton icon="mdi-plus" @click="add">New policy</UiButton>
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="reload" />
    </template>
    <template #filters>
      <UiInput id="policy-search" v-model="query" label="Search" sr-only-label placeholder="Search policies" type="search" class="w-full md:max-w-sm" @enter="apply" />
    </template>
    <UiAlert v-if="error || page.error" kind="error" class="mb-3">{{ error || page.error }}</UiAlert>
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-12">
      <UiCard :padded="false" class="lg:col-span-7">
        <UiDataTable :items="page.items" :columns="columns" :loading="page.loading" :total="page.total" :page="lq.page.value" :page-size="lq.pageSize.value" :sort="lq.sort.value" caption="Policies" empty-title="No policies" clickable @row-click="select" @update:page="lq.setPage" @update:page-size="lq.setPageSize" @update:sort="lq.setSort">
          <template #cell-status="{ row }"><UiStatusChip :status="row.status" /></template>
          <template #actions="{ row }">
            <UiButton size="xs" variant="text" icon="mdi-pencil-outline" icon-only label="Edit" @click="edit(row)" />
            <UiButton size="xs" variant="text" icon="mdi-delete-outline" icon-only label="Delete" @click="remove(row)" />
          </template>
        </UiDataTable>
      </UiCard>
      <UiCard class="lg:col-span-5" :title="selected ? 'Covered assets — ' + selected.name : 'Covered assets'">
        <p v-if="!selected" class="text-sm text-base-content/70">Select a policy to manage its covered assets.</p>
        <template v-else>
          <UiForm :form="coverForm" class="mb-3">
            <div class="grid grid-cols-1 gap-2 md:grid-cols-12 md:items-end">
              <div class="md:col-span-7"><UiCombobox v-bind="coverForm.field('asset_id')" label="Asset" :options="assetOptions" required @search="assets.searchOptions" /></div>
              <div class="md:col-span-3"><UiNumberInput v-bind="coverForm.field('covered_value')" label="Covered value" :min="0" :step="0.01" /></div>
              <div class="md:col-span-2"><UiButton type="submit" block :loading="coverForm.submitting.value">Add</UiButton></div>
            </div>
          </UiForm>
          <UiAlert v-if="covered.error" kind="error" class="mb-3">{{ covered.error }}</UiAlert>
          <UiDataTable :items="covered.items" :columns="coveredColumns" :loading="covered.loading" :total="covered.total" :page="cq.page.value" :page-size="cq.pageSize.value" :sort="cq.sort.value" caption="Covered assets" empty-title="No assets covered" @update:page="cq.setPage" @update:page-size="cq.setPageSize" @update:sort="cq.setSort">
            <template #actions="{ row }"><UiButton size="xs" variant="text" icon="mdi-close" icon-only label="Remove" @click="uncover(row)" /></template>
          </UiDataTable>
        </template>
      </UiCard>
    </div>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit policy' : 'New policy'" :schema="insurancePolicySchema" :fields="fields" :initial="editing ?? { status: 'active', coverage_type: 'all_risk' }" :submit="submit" size="lg" @saved="reload" />
  </UiPage>
</template>
