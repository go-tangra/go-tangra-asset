<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { UiPage, UiAlert, UiCard, UiButton, UiStatGrid, UiStatTile, UiBarList, UiLiveIndicator, UiForm, UiFilePicker, UiSelect, UiBadge, UiEmptyState, type BarItem } from '@go-tangra/ui'
import { useZodForm } from '@go-tangra/ui/forms'
import { useStats } from '@/stores/stats'
import { useLive } from '@/stores/live'
import { describe } from '@/api/client'
import { backupImportSchema, BACKUP_MODES } from '@/schemas'

const stats = useStats()
const live = useLive()
const importResult = ref('')
const error = ref('')

let release: (() => void) | null = null
onMounted(() => {
  void stats.load()
  release = live.connect()
})
onUnmounted(() => release?.())

const s = computed(() => stats.snapshot)
const barColor: Record<string, BarItem['color']> = { deployable: 'success', assigned: 'info', broken: 'error', archived: 'neutral' }
const byStatus = computed<BarItem[]>(() => Object.entries(s.value?.assets_by_status ?? {}).sort((a, b) => b[1] - a[1]).map(([label, value]) => ({ label, value, color: barColor[label] ?? 'primary' })))

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
const importForm = useZodForm(backupImportSchema, {
  initial: { mode: 'skip' },
  onSubmit: async ({ file, mode }) => {
    let parsed: unknown
    try {
      parsed = JSON.parse(await file.text())
    } catch {
      importForm.setFieldError('file', 'The file is not valid JSON.')
      throw new Error('invalid json')
    }
    const res = await stats.importBackup(parsed as Record<string, unknown>, mode)
    importResult.value = 'Imported ' + JSON.stringify(res.imported) + ', skipped ' + JSON.stringify(res.skipped)
  },
  onSuccess: async () => {
    importForm.reset({ mode: 'skip' })
    await stats.load()
  },
})
</script>

<template>
  <UiPage title="Asset dashboard">
    <template #badges><UiLiveIndicator :connected="live.connected" /></template>
    <template #actions><UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="stats.load()" /></template>
    <UiAlert v-if="stats.error || error" kind="error" class="mb-3">{{ stats.error || error }}</UiAlert>
    <UiStatGrid class="mb-3" :cols="4">
      <UiStatTile title="Assets" :value="s?.total_assets ?? 0" icon="mdi-laptop" color="primary" :subtitle="(s?.assigned_assets ?? 0) + ' assigned'" />
      <UiStatTile title="Total purchase cost" :value="money(s?.total_cost)" icon="mdi-cash" color="secondary" />
      <UiStatTile title="Depreciated value" :value="money(s?.total_depreciated_value)" icon="mdi-trending-down" color="info" subtitle="double-declining balance" />
      <UiStatTile title="Expiring soon" :value="s?.expiring_soon ?? 0" icon="mdi-calendar-alert" color="warning" :subtitle="`warranty ${s?.warranty_expiring_soon ?? 0} · licenses ${s?.licenses_expiring_soon ?? 0} · insurance ${s?.insurance_expiring_soon ?? 0}`" />
    </UiStatGrid>
    <UiStatGrid class="mb-4" :cols="6">
      <UiStatTile title="Low stock" :value="s?.low_stock ?? 0" icon="mdi-package-down" color="error" />
      <UiStatTile title="Consumables" :value="s?.total_consumables ?? 0" icon="mdi-package-variant" />
      <UiStatTile title="Licenses" :value="s?.total_licenses ?? 0" icon="mdi-license" />
      <UiStatTile title="Policies" :value="s?.total_insurance_policies ?? 0" icon="mdi-shield-check-outline" />
      <UiStatTile title="Suppliers" :value="s?.total_suppliers ?? 0" icon="mdi-truck-outline" />
      <UiStatTile title="Locations" :value="s?.total_locations ?? 0" icon="mdi-map-marker-outline" />
    </UiStatGrid>
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-12">
      <UiCard title="Assets by status" class="lg:col-span-5">
        <UiBarList :items="byStatus" empty-title="No assets yet" />
      </UiCard>
      <div class="flex flex-col gap-4 lg:col-span-7">
        <UiCard title="Backup">
          <UiButton variant="soft" icon="mdi-download" class="mb-3" @click="exportBackup">Export tenant data</UiButton>
          <UiForm :form="importForm">
            <div class="grid grid-cols-1 gap-2 md:grid-cols-12 md:items-end">
              <div class="md:col-span-6"><UiFilePicker v-bind="importForm.field('file')" label="Backup file (.json)" accept="application/json" /></div>
              <div class="md:col-span-3"><UiSelect v-bind="importForm.field('mode')" label="On conflict" :options="BACKUP_MODES.map((m) => ({ title: m, value: m }))" :clearable="false" /></div>
              <div class="md:col-span-3"><UiButton type="submit" block :loading="importForm.submitting.value">Import</UiButton></div>
            </div>
          </UiForm>
          <p v-if="importResult" class="mt-2 text-sm">{{ importResult }}</p>
        </UiCard>
        <UiCard title="Recent lifecycle events">
          <UiEmptyState v-if="live.recent.length === 0" title="No events yet" />
          <ul v-else class="flex flex-col gap-1 text-sm">
            <li v-for="(e, i) in live.recent" :key="i" class="break-all"><UiBadge size="xs" class="me-2">{{ e.type }}</UiBadge>{{ new Date(e.at).toLocaleTimeString() }} — {{ JSON.stringify(e.data) }}</li>
          </ul>
        </UiCard>
      </div>
    </div>
  </UiPage>
</template>
