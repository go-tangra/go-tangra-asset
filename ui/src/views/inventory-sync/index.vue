<script setup lang="ts">
import { computed, ref } from 'vue'
import { useSync } from '@/stores/sync'

// Preview the inventory host ↔ asset diff, pick hosts, execute.
const sync = useSync()
const selected = ref<string[]>([])

const actionColor: Record<string, string> = { create: 'success', update: 'info', unchanged: 'grey' }
const actionable = computed(() => (sync.preview?.changes ?? []).filter((c) => c.action !== 'unchanged'))

async function preview(): Promise<void> {
  await sync.runPreview()
  selected.value = actionable.value.map((c) => c.hostname)
}
function toggleAll(): void {
  selected.value = selected.value.length === actionable.value.length ? [] : actionable.value.map((c) => c.hostname)
}
async function execute(): Promise<void> {
  await sync.execute(selected.value)
  if (!sync.error) await sync.runPreview()
}
function fmtChanges(c: Record<string, { old: string; new: string }> | undefined): string {
  if (!c) return ''
  return Object.entries(c)
    .map(([k, v]) => `${k}: ${v.old || '∅'} → ${v.new || '∅'}`)
    .join('; ')
}
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Inventory sync</h1>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-magnify-scan" :loading="sync.loading" class="me-2" @click="preview">Preview</v-btn>
      <v-btn color="success" prepend-icon="mdi-sync" :disabled="!sync.preview || selected.length === 0" :loading="sync.loading" @click="execute">Execute ({{ selected.length }})</v-btn>
    </div>
    <v-alert v-if="sync.error" type="error" variant="tonal" density="compact" class="mb-3">{{ sync.error }}</v-alert>
    <v-alert v-if="sync.result" type="success" variant="tonal" density="compact" class="mb-3">
      Created {{ sync.result.created }}, updated {{ sync.result.updated }}, skipped {{ sync.result.skipped }}
      <span v-if="sync.result.errors.length">; {{ sync.result.errors.length }} error(s): {{ sync.result.errors.join(', ') }}</span>
    </v-alert>
    <v-row v-if="sync.preview" dense class="mb-3">
      <v-col cols="6" md="3"><v-card variant="tonal"><v-card-text><div class="text-h5">{{ sync.preview.hosts }}</div><div class="text-body-2">Hosts</div></v-card-text></v-card></v-col>
      <v-col cols="6" md="3"><v-card variant="tonal" color="success"><v-card-text><div class="text-h5">{{ sync.preview.create }}</div><div class="text-body-2">To create</div></v-card-text></v-card></v-col>
      <v-col cols="6" md="3"><v-card variant="tonal" color="info"><v-card-text><div class="text-h5">{{ sync.preview.update }}</div><div class="text-body-2">To update</div></v-card-text></v-card></v-col>
      <v-col cols="6" md="3"><v-card variant="tonal"><v-card-text><div class="text-h5">{{ sync.preview.unchanged }}</div><div class="text-body-2">Unchanged</div></v-card-text></v-card></v-col>
    </v-row>
    <v-card v-if="sync.preview">
      <v-table density="compact">
        <thead>
          <tr>
            <th><v-checkbox-btn :model-value="selected.length > 0 && selected.length === actionable.length" :indeterminate="selected.length > 0 && selected.length < actionable.length" @click="toggleAll" /></th>
            <th>Hostname</th><th>Serial</th><th>Action</th><th>Asset</th><th>Changes</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="sync.preview.changes.length === 0"><td colspan="6" class="text-medium-emphasis">No inventory hosts for this tenant.</td></tr>
          <tr v-for="c in sync.preview.changes" :key="c.host_id">
            <td><v-checkbox-btn v-model="selected" :value="c.hostname" :disabled="c.action === 'unchanged'" /></td>
            <td class="font-weight-medium">{{ c.hostname }}</td>
            <td>{{ c.serial || '—' }}</td>
            <td><v-chip size="small" variant="tonal" :color="actionColor[c.action]">{{ c.action }}</v-chip></td>
            <td>{{ c.asset_tag || '—' }}</td>
            <td class="text-caption">{{ fmtChanges(c.changes) }}</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
    <div v-else class="text-medium-emphasis">Run a preview to see which inventory hosts would be created or updated as assets.</div>
  </div>
</template>
