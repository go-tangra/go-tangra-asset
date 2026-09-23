<script setup lang="ts">
import { computed, ref } from 'vue'
import { UiPage, UiAlert, UiCard, UiButton, UiDataTable, UiStatGrid, UiStatTile, UiStatusChip, UiEmptyState, type Column } from '@freya/ui'
import { useSync } from '@/stores/sync'
import type { SyncChange } from '@/api/types'

// Preview the inventory host ↔ asset diff, pick hosts, execute.
const sync = useSync()
const selected = ref<string[]>([])
const actionable = computed(() => (sync.preview?.changes ?? []).filter((c) => c.action !== 'unchanged'))
const rows = computed(() => (sync.preview?.changes ?? []).map((c) => ({ ...c, id: c.hostname })))

async function preview(): Promise<void> {
  await sync.runPreview()
  selected.value = actionable.value.map((c) => c.hostname)
}
async function execute(): Promise<void> {
  await sync.execute(selected.value.filter((h) => actionable.value.some((c) => c.hostname === h)))
  if (!sync.error) await sync.runPreview()
}
function fmtChanges(c: Record<string, { old: string; new: string }> | undefined): string {
  if (!c) return ''
  return Object.entries(c)
    .map(([k, v]) => `${k}: ${v.old || '∅'} → ${v.new || '∅'}`)
    .join('; ')
}
const columns: Column<SyncChange & { id: string }>[] = [
  { key: 'hostname', label: 'Hostname', sortable: true },
  { key: 'serial', label: 'Serial', hideOnStack: true },
  { key: 'action', label: 'Action', width: 'sm', sortable: true },
  { key: 'asset_tag', label: 'Asset' },
  { key: 'changes', label: 'Changes', format: (c) => fmtChanges(c.changes) },
]
const selectable = computed(() => selected.value.filter((h) => actionable.value.some((c) => c.hostname === h)))
</script>

<template>
  <UiPage title="Inventory sync" subtitle="Create or update assets from the hosts the inventory module knows">
    <template #actions>
      <UiButton icon="mdi-magnify-scan" variant="soft" :loading="sync.loading" @click="preview">Preview</UiButton>
      <UiButton icon="mdi-sync" color="success" :disabled="!sync.preview || selectable.length === 0" :loading="sync.loading" @click="execute">Execute ({{ selectable.length }})</UiButton>
    </template>
    <UiAlert v-if="sync.error" kind="error" class="mb-3">{{ sync.error }}</UiAlert>
    <UiAlert v-if="sync.result" kind="success" class="mb-3">
      Created {{ sync.result.created }}, updated {{ sync.result.updated }}, skipped {{ sync.result.skipped }}
      <span v-if="sync.result.errors.length">; {{ sync.result.errors.length }} error(s): {{ sync.result.errors.join(', ') }}</span>
    </UiAlert>
    <template v-if="sync.preview">
      <UiStatGrid class="mb-4" :cols="4">
        <UiStatTile title="Hosts" :value="sync.preview.hosts" icon="mdi-server" />
        <UiStatTile title="To create" :value="sync.preview.create" icon="mdi-plus-box-outline" color="success" />
        <UiStatTile title="To update" :value="sync.preview.update" icon="mdi-update" color="info" />
        <UiStatTile title="Unchanged" :value="sync.preview.unchanged" icon="mdi-check" />
      </UiStatGrid>
      <UiCard :padded="false">
        <UiDataTable v-model:selected="selected" :items="rows" :columns="columns" caption="Inventory hosts" empty-title="No inventory hosts for this tenant" selectable>
          <template #cell-action="{ row }"><UiStatusChip :status="row.action" :colors="{ create: 'success', update: 'info', unchanged: 'neutral' }" /></template>
        </UiDataTable>
      </UiCard>
    </template>
    <UiEmptyState v-else title="No preview yet" text="Run a preview to see which inventory hosts would be created or updated as assets." icon="mdi-magnify-scan" />
  </UiPage>
</template>
