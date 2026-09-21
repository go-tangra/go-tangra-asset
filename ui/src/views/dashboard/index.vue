<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useStats } from '@/stores/stats'
import { useLive } from '@/stores/live'
import { describe } from '@/api/client'
import StatsCard from '@/components/StatsCard.vue'

const stats = useStats()
const live = useLive()
const importFile = ref<File | null>(null)
const importMode = ref<'skip' | 'overwrite'>('skip')
const importResult = ref('')
const error = ref('')

let release: (() => void) | null = null
onMounted(() => {
  void stats.load()
  release = live.connect()
})
onUnmounted(() => release?.())

const s = computed(() => stats.snapshot)
const byStatus = computed<[string, number][]>(() => Object.entries(s.value?.assets_by_status ?? {}).sort((a, b) => b[1] - a[1]))
const statusMax = computed(() => Math.max(1, ...byStatus.value.map(([, n]) => n)))
const statusColor: Record<string, string> = { deployable: 'success', assigned: 'info', broken: 'error', archived: 'grey' }

function money(v?: number): string {
  return (v ?? 0).toLocaleString(undefined, { maximumFractionDigits: 2 })
}

async function exportBackup(): Promise<void> {
  error.value = ''
  try {
    const b = await stats.exportBackup()
    const blob = new Blob([JSON.stringify(b, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'asset-backup-' + new Date().toISOString().slice(0, 10) + '.json'
    a.click()
    URL.revokeObjectURL(a.href)
  } catch (e) {
    error.value = describe(e)
  }
}
async function importBackup(): Promise<void> {
  if (!importFile.value) return
  error.value = ''
  importResult.value = ''
  try {
    const text = await importFile.value.text()
    const res = await stats.importBackup(JSON.parse(text), importMode.value)
    importResult.value = 'Imported ' + JSON.stringify(res.imported) + ', skipped ' + JSON.stringify(res.skipped)
    importFile.value = null
    await stats.load()
  } catch (e) {
    error.value = e instanceof SyntaxError ? 'The file is not valid JSON.' : describe(e)
  }
}
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Asset dashboard</h1>
      <v-chip v-if="live.connected" size="x-small" color="success" variant="tonal" class="ms-3">live</v-chip>
      <v-spacer />
      <v-btn variant="text" icon="mdi-refresh" @click="stats.load()" />
    </div>
    <v-alert v-if="stats.error || error" type="error" variant="tonal" density="compact" class="mb-3">{{ stats.error || error }}</v-alert>
    <v-row dense class="mb-2">
      <v-col cols="6" md="3"><StatsCard title="Assets" :value="s?.total_assets ?? 0" icon="mdi-laptop" color="primary" :subtitle="(s?.assigned_assets ?? 0) + ' assigned'" /></v-col>
      <v-col cols="6" md="3"><StatsCard title="Total purchase cost" :value="money(s?.total_cost)" icon="mdi-cash" color="secondary" /></v-col>
      <v-col cols="6" md="3"><StatsCard title="Depreciated value" :value="money(s?.total_depreciated_value)" icon="mdi-trending-down" color="info" subtitle="double-declining balance" /></v-col>
      <v-col cols="6" md="3"><StatsCard title="Expiring soon" :value="s?.expiring_soon ?? 0" icon="mdi-calendar-alert" color="warning" :subtitle="`warranty ${s?.warranty_expiring_soon ?? 0} · licenses ${s?.licenses_expiring_soon ?? 0} · insurance ${s?.insurance_expiring_soon ?? 0}`" /></v-col>
    </v-row>
    <v-row dense class="mb-4">
      <v-col cols="6" md="2"><StatsCard title="Low stock" :value="s?.low_stock ?? 0" icon="mdi-package-down" color="error" /></v-col>
      <v-col cols="6" md="2"><StatsCard title="Consumables" :value="s?.total_consumables ?? 0" icon="mdi-package-variant" /></v-col>
      <v-col cols="6" md="2"><StatsCard title="Licenses" :value="s?.total_licenses ?? 0" icon="mdi-license" /></v-col>
      <v-col cols="6" md="2"><StatsCard title="Policies" :value="s?.total_insurance_policies ?? 0" icon="mdi-shield-check-outline" /></v-col>
      <v-col cols="6" md="2"><StatsCard title="Suppliers" :value="s?.total_suppliers ?? 0" icon="mdi-truck-outline" /></v-col>
      <v-col cols="6" md="2"><StatsCard title="Locations" :value="s?.total_locations ?? 0" icon="mdi-map-marker-outline" /></v-col>
    </v-row>
    <v-row>
      <v-col cols="12" md="5">
        <v-card variant="outlined">
          <v-card-title class="text-subtitle-1">Assets by status</v-card-title>
          <v-card-text>
            <div v-if="byStatus.length === 0" class="text-medium-emphasis">No assets yet.</div>
            <div v-for="[st, n] in byStatus" :key="st" class="mb-2">
              <div class="d-flex justify-space-between text-body-2"><span>{{ st }}</span><span>{{ n }}</span></div>
              <v-progress-linear :model-value="(n / statusMax) * 100" :color="statusColor[st] ?? 'primary'" height="8" rounded />
            </div>
          </v-card-text>
        </v-card>
      </v-col>
      <v-col cols="12" md="7">
        <v-card variant="outlined" class="mb-4">
          <v-card-title class="text-subtitle-1">Backup</v-card-title>
          <v-card-text>
            <v-btn variant="tonal" prepend-icon="mdi-download" class="mb-3" @click="exportBackup">Export tenant data</v-btn>
            <v-row dense align="center">
              <v-col cols="12" md="6"><v-file-input v-model="importFile" label="Backup file (.json)" accept="application/json" density="comfortable" /></v-col>
              <v-col cols="6" md="3"><v-select v-model="importMode" :items="['skip', 'overwrite']" label="On conflict" density="comfortable" /></v-col>
              <v-col cols="6" md="3"><v-btn block color="primary" :disabled="!importFile" @click="importBackup">Import</v-btn></v-col>
            </v-row>
            <div v-if="importResult" class="text-body-2 mt-2">{{ importResult }}</div>
          </v-card-text>
        </v-card>
        <v-card variant="outlined">
          <v-card-title class="text-subtitle-1">Recent lifecycle events</v-card-title>
          <v-card-text>
            <div v-if="live.recent.length === 0" class="text-medium-emphasis">No events yet.</div>
            <div v-for="(e, i) in live.recent" :key="i" class="text-body-2"><v-chip size="x-small" variant="tonal" class="me-2">{{ e.type }}</v-chip>{{ new Date(e.at).toLocaleTimeString() }} — {{ JSON.stringify(e.data) }}</div>
          </v-card-text>
        </v-card>
      </v-col>
    </v-row>
  </div>
</template>
