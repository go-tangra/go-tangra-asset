<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { UiPage, UiAlert, UiCard, UiButton, UiStatusChip, UiKeyValueTable, UiDataTable, UiDocumentList, UiForm, UiCombobox, UiSelect, UiInput, UiFilePicker, UiBadge, UiDrawer, UiRecordDrawer, useConfirm, type Column } from '@go-tangra/ui'
import { zodToFields, useZodForm } from '@go-tangra/ui/forms'
import { useAssets } from '@/stores/assets'
import { useOrg } from '@/stores/org'
import { useDocuments } from '@/stores/documents'
import { api, describe } from '@/api/client'
import { assetSchema, assignSchema, unassignSchema, ASSET_STATUSES, type AssetInput } from '@/schemas'
import type { Asset, Assignment } from '@/api/types'

const route = useRoute()
const router = useRouter()
const store = useAssets()
const org = useOrg()
const docs = useDocuments()
const confirm = useConfirm()

const id = String(route.params.id)
const asset = ref<Asset | null>(null)
const history = ref<Assignment[]>([])
const error = ref('')
const edit = ref(false)
const assignDialog = ref(false)
const unassignDialog = ref(false)
const photoFile = ref<File | null>(null)
const photoBust = ref(String(Date.now()))

async function reload(): Promise<void> {
  error.value = ''
  try {
    asset.value = await store.get(id)
    history.value = await store.history(id)
  } catch (e) {
    error.value = describe(e)
  }
}
onMounted(() => {
  void reload()
  void org.loadCategories()
  void org.loadSuppliers()
  void org.loadLocations()
  void store.loadUsers()
})

const fields = computed(() =>
  zodToFields(assetSchema, {
    model_name: { label: 'Model' },
    category_id: { type: 'select', options: org.categories.map((c) => ({ title: c.name, value: c.id })) },
    supplier_id: { type: 'select', options: org.suppliers.map((s) => ({ title: s.name, value: s.id })) },
    location_id: { type: 'select', options: org.locations.map((l) => ({ title: l.path || l.name, value: l.id })) },
    status: { type: 'select', options: ASSET_STATUSES.map((s) => ({ title: s, value: s })), hint: 'Assigned is set through Assign' },
    warranty_months: { label: 'Warranty (months)' },
    useful_life_years: { label: 'Useful life (years)' },
    depreciation_rate: { label: 'Depreciation rate (0–1)' },
  }),
)
const locationOptions = computed(() => org.locations.map((l) => ({ title: l.path || l.name, value: l.id })))
const userOptions = computed(() => store.users.map((u) => ({ title: u.display_name, value: u.id })))

const assignForm = useZodForm(assignSchema, {
  initial: { user_id: '', notes: '' },
  onSubmit: async (v) => {
    asset.value = await store.assign(id, v.user_id, v.notes ?? '')
  },
  onSuccess: async () => {
    assignDialog.value = false
    assignForm.reset({ user_id: '', notes: '' })
    history.value = await store.history(id)
  },
})
const unassignForm = useZodForm(unassignSchema, {
  initial: { location_id: '', notes: '' },
  onSubmit: async (v) => {
    asset.value = await store.unassign(id, v.location_id ?? '', v.notes ?? '')
  },
  onSuccess: async () => {
    unassignDialog.value = false
    unassignForm.reset({ location_id: '', notes: '' })
    history.value = await store.history(id)
  },
})
async function remove(): Promise<void> {
  if (!(await confirm.ask({ title: `Delete ${asset.value?.asset_tag ?? 'this asset'}?`, text: 'Documents and history are removed with it.', danger: true, confirmLabel: 'Delete' }))) return
  try {
    await store.remove(id)
    void router.push({ name: 'asset-assets' })
  } catch (e) {
    error.value = describe(e)
  }
}
async function uploadPhoto(): Promise<void> {
  if (!photoFile.value) return
  try {
    await docs.uploadPhoto(id, photoFile.value)
    photoFile.value = null
    photoBust.value = String(Date.now())
    await reload()
  } catch (e) {
    error.value = describe(e)
  }
}
async function deletePhoto(): Promise<void> {
  try {
    await store.deletePhoto(id)
    await reload()
  } catch (e) {
    error.value = describe(e)
  }
}

function money(v?: number): string {
  return v === undefined ? '—' : v.toLocaleString(undefined, { maximumFractionDigits: 2 })
}
function fmt(ts?: string): string {
  return ts ? new Date(ts).toLocaleString() : ''
}
const warrantyEnd = computed(() => {
  const a = asset.value
  if (!a?.purchase_date || !a.warranty_months) return ''
  const d = new Date(a.purchase_date)
  d.setMonth(d.getMonth() + a.warranty_months)
  return d.toLocaleDateString()
})
const details = computed(() => {
  const a = asset.value
  if (!a) return []
  return [
    { label: 'Serial', value: a.serial },
    { label: 'Model', value: [a.model_name, a.model_number].filter(Boolean).join(' ') },
    { label: 'Category', value: org.categoryName(a.category_id) },
    { label: 'Supplier', value: org.supplierName(a.supplier_id) },
    { label: 'Location', value: org.locationName(a.location_id) },
    { label: 'Assignee', value: a.assignee_name || a.user_id },
    { label: 'Purchased', value: a.purchase_date ? new Date(a.purchase_date).toLocaleDateString() : '' },
    { label: 'Order', value: a.order_number },
    { label: 'Updated', value: fmt(a.updated_at) },
    { label: 'Notes', value: a.notes },
  ]
})
const depreciation = computed(() => {
  const a = asset.value
  if (!a) return []
  return [
    { label: 'Purchase cost', value: money(a.purchase_cost) },
    { label: 'Current book value', value: money(a.book_value) },
    { label: 'Salvage value', value: money(a.salvage_value) },
    { label: 'Useful life', value: a.useful_life_years ? a.useful_life_years + ' y' : '' },
    { label: 'Rate (DDB)', value: a.depreciation_rate ?? 0.4 },
    { label: 'Warranty until', value: warrantyEnd.value },
  ]
})
const historyColumns: Column<Assignment>[] = [
  { key: 'action', label: 'Action', width: 'sm' },
  { key: 'user_name', label: 'User', format: (h) => h.user_name || h.user_id || '' },
  { key: 'assigned_at', label: 'At', format: (h) => fmt(h.assigned_at) },
  { key: 'returned_at', label: 'Returned', format: (h) => fmt(h.returned_at), hideOnStack: true },
  { key: 'assigned_by', label: 'By', hideOnStack: true },
  { key: 'notes', label: 'Notes' },
]
</script>

<template>
  <UiPage :title="asset?.asset_tag ?? 'Asset'" :subtitle="asset?.name">
    <template #before-title><UiButton variant="text" icon="mdi-arrow-left" icon-only label="Back to assets" @click="router.push({ name: 'asset-assets' })" /></template>
    <template #badges><UiStatusChip v-if="asset" :status="asset.status" /></template>
    <template #actions>
      <UiButton v-if="asset?.status === 'deployable'" icon="mdi-account-arrow-right" @click="assignDialog = true">Assign</UiButton>
      <UiButton v-if="asset?.status === 'assigned'" color="warning" icon="mdi-account-arrow-left" @click="unassignDialog = true">Unassign</UiButton>
      <UiButton variant="soft" icon="mdi-pencil-outline" @click="edit = true">Edit</UiButton>
      <UiButton variant="text" color="error" icon="mdi-delete-outline" icon-only label="Delete asset" @click="remove" />
    </template>
    <UiAlert v-if="error" kind="error" class="mb-3">{{ error }}</UiAlert>
    <div v-if="asset" class="grid grid-cols-1 gap-4 lg:grid-cols-12">
      <div class="flex flex-col gap-4 lg:col-span-4">
        <UiCard title="Photo">
          <img v-if="asset.has_photo" :src="docs.photoUrl(id, photoBust)" :alt="'Photo of ' + asset.asset_tag" class="mb-2 max-h-60 w-full rounded-box object-contain">
          <p v-else class="mb-2 text-sm text-base-content/70">No photo.</p>
          <UiFilePicker id="asset-photo" v-model="photoFile" label="Upload photo" accept="image/png,image/jpeg,image/gif,image/webp" />
          <div class="mt-2 flex gap-2">
            <UiButton size="sm" :disabled="!photoFile" @click="uploadPhoto">Upload</UiButton>
            <UiButton v-if="asset.has_photo" size="sm" variant="text" color="error" @click="deletePhoto">Remove</UiButton>
          </div>
        </UiCard>
        <UiCard title="Depreciation"><UiKeyValueTable :items="depreciation" /></UiCard>
      </div>
      <div class="flex flex-col gap-4 lg:col-span-8">
        <UiCard title="Details">
          <UiKeyValueTable :items="details" :columns="2" />
          <div v-if="asset.tags && Object.keys(asset.tags).length" class="mt-3 flex flex-wrap gap-1">
            <UiBadge v-for="(v, k) in asset.tags" :key="k">{{ k }}={{ v }}</UiBadge>
          </div>
        </UiCard>
        <UiCard title="Assignment history" :padded="false">
          <UiDataTable :items="history" :columns="historyColumns" caption="Assignment history" empty-title="Never assigned">
            <template #cell-action="{ row }"><UiStatusChip :status="row.action" :colors="{ assigned: 'info', unassigned: 'neutral', transferred: 'info' }" /></template>
          </UiDataTable>
        </UiCard>
        <UiCard><UiDocumentList :api="api" :base="'assets/' + id + '/documents'" /></UiCard>
      </div>
    </div>

    <UiRecordDrawer v-model="edit" close-on-save title="Edit asset" :schema="assetSchema" :fields="fields" :initial="asset ?? undefined" :submit="(v) => store.update(id, v as AssetInput)" size="xl" @saved="reload" />

    <UiDrawer v-model="assignDialog" title="Assign asset" size="md">
      <UiForm :form="assignForm">
        <UiCombobox v-bind="assignForm.field('user_id')" label="User" :options="userOptions" required />
        <UiInput v-bind="assignForm.field('notes')" label="Notes" class="mt-2" />
      </UiForm>
      <template #actions>
        <UiButton variant="text" @click="assignDialog = false">Cancel</UiButton>
        <UiButton :loading="assignForm.submitting.value" @click="assignForm.submit()">Assign</UiButton>
      </template>
    </UiDrawer>
    <UiDrawer v-model="unassignDialog" title="Unassign asset" size="md">
      <UiForm :form="unassignForm">
        <UiSelect v-bind="unassignForm.field('location_id')" label="Return to location (optional)" :options="locationOptions" />
        <UiInput v-bind="unassignForm.field('notes')" label="Notes" class="mt-2" />
      </UiForm>
      <template #actions>
        <UiButton variant="text" @click="unassignDialog = false">Cancel</UiButton>
        <UiButton color="warning" :loading="unassignForm.submitting.value" @click="unassignForm.submit()">Unassign</UiButton>
      </template>
    </UiDrawer>
  </UiPage>
</template>
