<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { UiPage, UiAlert, UiCard, UiInput, UiButton, UiDataTable, UiStatusChip, UiForm, UiCombobox, UiNumberInput, UiRecordDrawer, useConfirm, type Column } from '@go-tangra/ui'
import { zodToFields, useZodForm } from '@go-tangra/ui/forms'
import { useInventories } from '@/stores/inventories'
import { useAssets } from '@/stores/assets'
import { describe } from '@/api/client'
import { insurancePolicySchema, policyAssetSchema, POLICY_STATUSES, COVERAGE_TYPES } from '@/schemas'
import type { InsurancePolicy, PolicyAsset } from '@/api/types'

const inv = useInventories()
const assets = useAssets()
const confirm = useConfirm()
const query = ref('')
const dialog = ref(false)
const editing = ref<InsurancePolicy | null>(null)
const selected = ref<InsurancePolicy | null>(null)
const covered = ref<PolicyAsset[]>([])
const error = ref('')

onMounted(() => {
  void inv.loadPolicies()
  void assets.list()
})

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
  { key: 'asset_tag', label: 'Tag' },
  { key: 'asset_name', label: 'Name' },
  { key: 'covered_value', label: 'Covered', align: 'end', format: (pa) => String(pa.covered_value ?? 0) },
]

async function select(p: InsurancePolicy): Promise<void> {
  selected.value = p
  error.value = ''
  try {
    covered.value = await inv.policyAssets(p.id)
  } catch (e) {
    error.value = describe(e)
  }
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
    await inv.loadPolicies(query.value)
  } catch (e) {
    error.value = describe(e)
  }
}
const coverForm = useZodForm(policyAssetSchema, {
  initial: { asset_id: '', covered_value: 0 },
  onSubmit: (v) => inv.addPolicyAsset(selected.value!.id, v.asset_id, v.covered_value),
  onSuccess: async () => {
    coverForm.reset({ asset_id: '', covered_value: 0 })
    await select(selected.value!)
    await inv.loadPolicies(query.value)
  },
})
async function uncover(pa: PolicyAsset): Promise<void> {
  if (!selected.value) return
  try {
    await inv.removePolicyAsset(selected.value.id, pa.asset_id)
    await select(selected.value)
    await inv.loadPolicies(query.value)
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
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="inv.loadPolicies(query)" />
    </template>
    <template #filters>
      <UiInput id="policy-search" v-model="query" label="Search" sr-only-label placeholder="Search policies" type="search" class="w-full md:max-w-sm" @enter="inv.loadPolicies(query)" />
    </template>
    <UiAlert v-if="error || inv.error" kind="error" class="mb-3">{{ error || inv.error }}</UiAlert>
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-12">
      <UiCard :padded="false" class="lg:col-span-7">
        <UiDataTable :items="inv.policies" :columns="columns" caption="Policies" empty-title="No policies" clickable @row-click="select">
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
              <div class="md:col-span-7"><UiCombobox v-bind="coverForm.field('asset_id')" label="Asset" :options="assets.items.map((a) => ({ title: a.asset_tag + ' · ' + (a.name || ''), value: a.id }))" required /></div>
              <div class="md:col-span-3"><UiNumberInput v-bind="coverForm.field('covered_value')" label="Covered value" :min="0" :step="0.01" /></div>
              <div class="md:col-span-2"><UiButton type="submit" block :loading="coverForm.submitting.value">Add</UiButton></div>
            </div>
          </UiForm>
          <UiDataTable :items="covered" :columns="coveredColumns" caption="Covered assets" empty-title="No assets covered">
            <template #actions="{ row }"><UiButton size="xs" variant="text" icon="mdi-close" icon-only label="Remove" @click="uncover(row)" /></template>
          </UiDataTable>
        </template>
      </UiCard>
    </div>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit policy' : 'New policy'" :schema="insurancePolicySchema" :fields="fields" :initial="editing ?? { status: 'active', coverage_type: 'all_risk' }" :submit="submit" size="lg" @saved="inv.loadPolicies(query)" />
  </UiPage>
</template>
