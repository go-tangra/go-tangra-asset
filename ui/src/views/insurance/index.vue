<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useInventories } from '@/stores/inventories'
import { useAssets } from '@/stores/assets'
import { describe } from '@/api/client'
import RecordDialog from '@/components/RecordDialog.vue'
import type { Field } from '@/components/RecordDialog.vue'
import type { InsurancePolicy, PolicyAsset } from '@/api/types'

const inv = useInventories()
const assets = useAssets()
const query = ref('')
const dialog = ref(false)
const editing = ref<InsurancePolicy | null>(null)
const selected = ref<InsurancePolicy | null>(null)
const covered = ref<PolicyAsset[]>([])
const addAsset = ref<string | null>(null)
const coveredValue = ref(0)
const error = ref('')

onMounted(() => {
  void inv.loadPolicies()
  void assets.list()
})

const fields: Field[] = [
  { key: 'name', label: 'Name', required: true },
  { key: 'policy_number', label: 'Policy number', required: true },
  { key: 'provider', label: 'Provider' },
  { key: 'coverage_type', label: 'Coverage type', type: 'select', options: ['all_risk', 'fire_theft', 'liability', 'equipment_breakdown', 'cyber'].map((s) => ({ title: s, value: s })) },
  { key: 'status', label: 'Status', type: 'select', options: ['active', 'expired', 'cancelled'].map((s) => ({ title: s, value: s })) },
  { key: 'premium_amount', label: 'Premium', type: 'number' },
  { key: 'deductible', label: 'Deductible', type: 'number' },
  { key: 'coverage_limit', label: 'Coverage limit', type: 'number' },
  { key: 'valid_from', label: 'Valid from', type: 'date' },
  { key: 'valid_to', label: 'Valid to', type: 'date' },
  { key: 'notes', label: 'Notes', type: 'textarea', cols: 12 },
]
const statusColor: Record<string, string> = { active: 'success', expired: 'error', cancelled: 'grey' }

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
  error.value = ''
  try {
    await inv.removePolicy(p.id)
    if (selected.value?.id === p.id) selected.value = null
    await inv.loadPolicies(query.value)
  } catch (e) {
    error.value = describe(e)
  }
}
async function cover(): Promise<void> {
  if (!selected.value || !addAsset.value) return
  error.value = ''
  try {
    await inv.addPolicyAsset(selected.value.id, addAsset.value, Number(coveredValue.value))
    addAsset.value = null
    coveredValue.value = 0
    await select(selected.value)
    await inv.loadPolicies(query.value)
  } catch (e) {
    error.value = describe(e)
  }
}
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
function d(ts?: string): string {
  return ts ? new Date(ts).toLocaleDateString() : '—'
}
const submit = (v: Record<string, unknown>) => (editing.value ? inv.updatePolicy(editing.value.id, v) : inv.createPolicy(v))
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Insurance policies</h1>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-plus" class="me-2" @click="add">New policy</v-btn>
      <v-btn variant="text" icon="mdi-refresh" @click="inv.loadPolicies(query)" />
    </div>
    <v-text-field v-model="query" label="Search" density="comfortable" clearable class="mb-2" @keyup.enter="inv.loadPolicies(query)" />
    <v-alert v-if="error || inv.error" type="error" variant="tonal" density="compact" class="mb-3">{{ error || inv.error }}</v-alert>
    <v-row>
      <v-col cols="12" md="7">
        <v-card>
          <v-table hover>
            <thead><tr><th>Name</th><th>Number</th><th>Provider</th><th>Valid to</th><th>Status</th><th class="text-right">Assets</th><th /></tr></thead>
            <tbody>
              <tr v-if="inv.policies.length === 0"><td colspan="7" class="text-medium-emphasis">No policies.</td></tr>
              <tr v-for="p in inv.policies" :key="p.id" style="cursor: pointer" :class="{ 'bg-surface-variant': selected?.id === p.id }" @click="select(p)">
                <td class="font-weight-medium">{{ p.name }}</td>
                <td>{{ p.policy_number }}</td>
                <td>{{ p.provider || '—' }}</td>
                <td>{{ d(p.valid_to) }}</td>
                <td><v-chip size="small" variant="tonal" :color="statusColor[p.status]">{{ p.status }}</v-chip></td>
                <td class="text-right">{{ p.asset_count }}</td>
                <td class="text-right">
                  <v-btn icon="mdi-pencil-outline" size="small" variant="text" @click.stop="edit(p)" />
                  <v-btn icon="mdi-delete-outline" size="small" variant="text" @click.stop="remove(p)" />
                </td>
              </tr>
            </tbody>
          </v-table>
        </v-card>
      </v-col>
      <v-col cols="12" md="5">
        <v-card v-if="selected" variant="outlined">
          <v-card-title class="text-subtitle-1">Covered assets — {{ selected.name }}</v-card-title>
          <v-card-text>
            <v-row dense align="center">
              <v-col cols="12" md="7"><v-autocomplete v-model="addAsset" :items="assets.items.map((a) => ({ title: a.asset_tag + ' · ' + (a.name || ''), value: a.id }))" label="Asset" density="comfortable" /></v-col>
              <v-col cols="8" md="3"><v-text-field v-model="coveredValue" type="number" label="Covered value" density="comfortable" /></v-col>
              <v-col cols="4" md="2"><v-btn block color="primary" :disabled="!addAsset" @click="cover">Add</v-btn></v-col>
            </v-row>
            <v-table density="compact">
              <thead><tr><th>Tag</th><th>Name</th><th class="text-right">Covered</th><th /></tr></thead>
              <tbody>
                <tr v-if="covered.length === 0"><td colspan="4" class="text-medium-emphasis">No assets covered.</td></tr>
                <tr v-for="pa in covered" :key="pa.id">
                  <td>{{ pa.asset_tag }}</td>
                  <td>{{ pa.asset_name }}</td>
                  <td class="text-right">{{ pa.covered_value ?? 0 }}</td>
                  <td class="text-right"><v-btn icon="mdi-close" size="x-small" variant="text" @click="uncover(pa)" /></td>
                </tr>
              </tbody>
            </v-table>
          </v-card-text>
        </v-card>
        <div v-else class="text-medium-emphasis pa-4">Select a policy to manage its covered assets.</div>
      </v-col>
    </v-row>
    <RecordDialog v-model="dialog" :title="editing ? 'Edit policy' : 'New policy'" :fields="fields" :initial="editing ?? { status: 'active', coverage_type: 'all_risk' }" :submit="submit" @saved="inv.loadPolicies(query)" />
  </div>
</template>
