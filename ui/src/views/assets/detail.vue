<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAssets } from '@/stores/assets'
import { useOrg } from '@/stores/org'
import { useDocuments } from '@/stores/documents'
import { describe } from '@/api/client'
import RecordDialog from '@/components/RecordDialog.vue'
import EntityDocuments from '@/components/EntityDocuments.vue'
import type { Field } from '@/components/RecordDialog.vue'
import type { Asset, AssetInput, Assignment } from '@/api/types'

const route = useRoute()
const router = useRouter()
const store = useAssets()
const org = useOrg()
const docs = useDocuments()

const id = String(route.params.id)
const asset = ref<Asset | null>(null)
const history = ref<Assignment[]>([])
const error = ref('')
const edit = ref(false)
const assignDialog = ref(false)
const unassignDialog = ref(false)
const assignee = ref<string | null>(null)
const returnLocation = ref<string | null>(null)
const notes = ref('')
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

const fields = computed<Field[]>(() => [
  { key: 'name', label: 'Name', required: true },
  { key: 'asset_tag', label: 'Asset tag' },
  { key: 'serial', label: 'Serial' },
  { key: 'model_name', label: 'Model' },
  { key: 'model_number', label: 'Model number' },
  { key: 'category_id', label: 'Category', type: 'select', options: org.categories.map((c) => ({ title: c.name, value: c.id })) },
  { key: 'supplier_id', label: 'Supplier', type: 'select', options: org.suppliers.map((s) => ({ title: s.name, value: s.id })) },
  { key: 'location_id', label: 'Location', type: 'select', options: org.locations.map((l) => ({ title: l.path || l.name, value: l.id })) },
  { key: 'status', label: 'Status', type: 'select', options: ['deployable', 'broken', 'archived'].map((s) => ({ title: s, value: s })), hint: 'Assigned is set through Assign' },
  { key: 'purchase_date', label: 'Purchase date', type: 'date' },
  { key: 'purchase_cost', label: 'Purchase cost', type: 'number' },
  { key: 'order_number', label: 'Order number' },
  { key: 'warranty_months', label: 'Warranty (months)', type: 'number' },
  { key: 'useful_life_years', label: 'Useful life (years)', type: 'number' },
  { key: 'salvage_value', label: 'Salvage value', type: 'number' },
  { key: 'depreciation_rate', label: 'Depreciation rate (0–1)', type: 'number' },
  { key: 'notes', label: 'Notes', type: 'textarea', cols: 12 },
])

const statusColor: Record<string, string> = { deployable: 'success', assigned: 'info', broken: 'error', archived: 'grey' }

async function doAssign(): Promise<void> {
  if (!assignee.value) return
  try {
    asset.value = await store.assign(id, assignee.value, notes.value)
    assignDialog.value = false
    notes.value = ''
    history.value = await store.history(id)
  } catch (e) {
    error.value = describe(e)
  }
}
async function doUnassign(): Promise<void> {
  try {
    asset.value = await store.unassign(id, returnLocation.value ?? '', notes.value)
    unassignDialog.value = false
    notes.value = ''
    history.value = await store.history(id)
  } catch (e) {
    error.value = describe(e)
  }
}
async function remove(): Promise<void> {
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
  return ts ? new Date(ts).toLocaleString() : '—'
}
const warrantyEnd = computed(() => {
  const a = asset.value
  if (!a?.purchase_date || !a.warranty_months) return ''
  const d = new Date(a.purchase_date)
  d.setMonth(d.getMonth() + a.warranty_months)
  return d.toLocaleDateString()
})
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <v-btn variant="text" icon="mdi-arrow-left" class="me-2" @click="router.push({ name: 'asset-assets' })" />
      <h1 class="text-h5">{{ asset?.asset_tag ?? 'Asset' }} <span class="text-medium-emphasis">— {{ asset?.name }}</span></h1>
      <v-chip v-if="asset" size="small" :color="statusColor[asset.status]" variant="tonal" class="ms-3">{{ asset.status }}</v-chip>
      <v-spacer />
      <v-btn v-if="asset?.status === 'deployable'" color="primary" prepend-icon="mdi-account-arrow-right" class="me-2" @click="assignDialog = true">Assign</v-btn>
      <v-btn v-if="asset?.status === 'assigned'" color="warning" prepend-icon="mdi-account-arrow-left" class="me-2" @click="unassignDialog = true">Unassign</v-btn>
      <v-btn variant="tonal" prepend-icon="mdi-pencil-outline" class="me-2" @click="edit = true">Edit</v-btn>
      <v-btn variant="text" color="error" icon="mdi-delete-outline" @click="remove" />
    </div>
    <v-alert v-if="error" type="error" variant="tonal" density="compact" class="mb-3">{{ error }}</v-alert>
    <v-row v-if="asset">
      <v-col cols="12" md="4">
        <v-card variant="outlined" class="mb-4">
          <v-card-title class="text-subtitle-1">Photo</v-card-title>
          <v-card-text>
            <v-img v-if="asset.has_photo" :src="docs.photoUrl(id, photoBust)" max-height="240" class="mb-2 rounded" />
            <div v-else class="text-medium-emphasis mb-2">No photo.</div>
            <v-file-input v-model="photoFile" label="Upload photo" accept="image/png,image/jpeg,image/gif,image/webp" density="compact" prepend-icon="mdi-camera" />
            <div class="d-flex">
              <v-btn size="small" color="primary" :disabled="!photoFile" @click="uploadPhoto">Upload</v-btn>
              <v-btn v-if="asset.has_photo" size="small" variant="text" color="error" class="ms-2" @click="deletePhoto">Remove</v-btn>
            </div>
          </v-card-text>
        </v-card>
        <v-card variant="outlined">
          <v-card-title class="text-subtitle-1">Depreciation</v-card-title>
          <v-card-text>
            <div class="d-flex justify-space-between"><span>Purchase cost</span><b>{{ money(asset.purchase_cost) }}</b></div>
            <div class="d-flex justify-space-between"><span>Current book value</span><b>{{ money(asset.book_value) }}</b></div>
            <div class="d-flex justify-space-between"><span>Salvage value</span><span>{{ money(asset.salvage_value) }}</span></div>
            <div class="d-flex justify-space-between"><span>Useful life</span><span>{{ asset.useful_life_years ? asset.useful_life_years + ' y' : '—' }}</span></div>
            <div class="d-flex justify-space-between"><span>Rate (DDB)</span><span>{{ asset.depreciation_rate ?? 0.4 }}</span></div>
            <div class="d-flex justify-space-between"><span>Warranty until</span><span>{{ warrantyEnd || '—' }}</span></div>
          </v-card-text>
        </v-card>
      </v-col>
      <v-col cols="12" md="8">
        <v-card variant="outlined" class="mb-4">
          <v-card-title class="text-subtitle-1">Details</v-card-title>
          <v-card-text>
            <v-row dense>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Serial</div>{{ asset.serial || '—' }}</v-col>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Model</div>{{ asset.model_name || '—' }} {{ asset.model_number }}</v-col>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Category</div>{{ org.categoryName(asset.category_id) || '—' }}</v-col>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Supplier</div>{{ org.supplierName(asset.supplier_id) || '—' }}</v-col>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Location</div>{{ org.locationName(asset.location_id) || '—' }}</v-col>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Assignee</div>{{ asset.assignee_name || asset.user_id || '—' }}</v-col>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Purchased</div>{{ asset.purchase_date ? new Date(asset.purchase_date).toLocaleDateString() : '—' }}</v-col>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Order</div>{{ asset.order_number || '—' }}</v-col>
              <v-col cols="6" md="4"><div class="text-caption text-medium-emphasis">Updated</div>{{ fmt(asset.updated_at) }}</v-col>
              <v-col cols="12"><div class="text-caption text-medium-emphasis">Notes</div>{{ asset.notes || '—' }}</v-col>
              <v-col v-if="asset.tags && Object.keys(asset.tags).length" cols="12">
                <v-chip v-for="(v, k) in asset.tags" :key="k" size="small" class="me-1 mb-1" variant="tonal">{{ k }}={{ v }}</v-chip>
              </v-col>
            </v-row>
          </v-card-text>
        </v-card>
        <v-card variant="outlined" class="mb-4">
          <v-card-title class="text-subtitle-1">Assignment history</v-card-title>
          <v-table density="compact">
            <thead><tr><th>Action</th><th>User</th><th>At</th><th>Returned</th><th>By</th><th>Notes</th></tr></thead>
            <tbody>
              <tr v-if="history.length === 0"><td colspan="6" class="text-medium-emphasis">Never assigned.</td></tr>
              <tr v-for="h in history" :key="h.id">
                <td><v-chip size="x-small" variant="tonal" :color="h.action === 'assigned' ? 'info' : 'grey'">{{ h.action }}</v-chip></td>
                <td>{{ h.user_name || h.user_id || '—' }}</td>
                <td>{{ fmt(h.assigned_at) }}</td>
                <td>{{ fmt(h.returned_at) }}</td>
                <td>{{ h.assigned_by || '—' }}</td>
                <td>{{ h.notes || '—' }}</td>
              </tr>
            </tbody>
          </v-table>
        </v-card>
        <EntityDocuments entity-type="asset" :entity-id="id" />
      </v-col>
    </v-row>

    <RecordDialog v-model="edit" title="Edit asset" :fields="fields" :initial="asset ?? undefined" :submit="(v) => store.update(id, v as unknown as AssetInput)" @saved="reload" />

    <v-dialog v-model="assignDialog" max-width="480">
      <v-card>
        <v-card-title>Assign asset</v-card-title>
        <v-card-text>
          <v-autocomplete v-model="assignee" :items="store.users.map((u) => ({ title: u.display_name, value: u.id }))" label="User" density="comfortable" />
          <v-text-field v-model="notes" label="Notes" density="comfortable" />
        </v-card-text>
        <v-card-actions><v-spacer /><v-btn variant="text" @click="assignDialog = false">Cancel</v-btn><v-btn color="primary" :disabled="!assignee" @click="doAssign">Assign</v-btn></v-card-actions>
      </v-card>
    </v-dialog>
    <v-dialog v-model="unassignDialog" max-width="480">
      <v-card>
        <v-card-title>Unassign asset</v-card-title>
        <v-card-text>
          <v-select v-model="returnLocation" :items="org.locations.map((l) => ({ title: l.path || l.name, value: l.id }))" label="Return to location (optional)" clearable density="comfortable" />
          <v-text-field v-model="notes" label="Notes" density="comfortable" />
        </v-card-text>
        <v-card-actions><v-spacer /><v-btn variant="text" @click="unassignDialog = false">Cancel</v-btn><v-btn color="warning" @click="doUnassign">Unassign</v-btn></v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>
