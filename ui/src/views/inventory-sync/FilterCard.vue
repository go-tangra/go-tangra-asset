<script setup lang="ts">
// Persisted inventory-sync filters (feature 030): matching hosts are neither
// created nor updated as assets, by the manual sync and the scheduled
// asset:inventory-sync task alike. Existing assets are never deleted.
import { onMounted, reactive, ref } from 'vue'
import { UiAlert, UiButton, UiCard, UiSwitch, UiTextarea } from '@go-tangra/ui'
import { useSync } from '@/stores/sync'
import { describe } from '@/api/client'

const emit = defineEmits<{ (e: 'saved'): void }>()
const sync = useSync()
const error = ref('')
const message = ref('')
const saving = ref(false)
const form = reactive({
  exclude_vms: false,
  exclude_containers: false,
  skip_stale: false,
  skip_retired: false,
  hostname_include: '',
  hostname_exclude: '',
  os_include: '',
  os_exclude: '',
})

const lines = (t: string): string[] => t.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
const text = (l: string[] | undefined): string => (l ?? []).join('\n')

function fill(): void {
  const s = sync.settings
  if (!s) return
  Object.assign(form, {
    exclude_vms: s.exclude_vms, exclude_containers: s.exclude_containers, skip_stale: s.skip_stale, skip_retired: s.skip_retired,
    hostname_include: text(s.hostname_include), hostname_exclude: text(s.hostname_exclude), os_include: text(s.os_include), os_exclude: text(s.os_exclude),
  })
}

onMounted(async () => {
  try {
    await sync.loadSettings()
    fill()
  } catch (e) {
    error.value = describe(e)
  }
})

async function save(): Promise<void> {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    await sync.saveSettings({
      exclude_vms: form.exclude_vms, exclude_containers: form.exclude_containers, skip_stale: form.skip_stale, skip_retired: form.skip_retired,
      hostname_include: lines(form.hostname_include), hostname_exclude: lines(form.hostname_exclude),
      os_include: lines(form.os_include), os_exclude: lines(form.os_exclude),
    })
    fill()
    message.value = 'Filter saved. It applies to previews, manual syncs and the scheduled sync.'
    emit('saved')
  } catch (e) {
    error.value = describe(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiCard title="Sync filter" data-test="sync-filter">
    <UiAlert v-if="error" kind="error" class="mb-3">{{ error }}</UiAlert>
    <UiAlert v-if="message" kind="success" class="mb-3" data-test="sync-filter-message">{{ message }}</UiAlert>
    <p class="mb-3 text-sm text-base-content/70">
      Hosts matching the filter are not imported or updated as assets. Assets that already exist are kept.
      Patterns are case-insensitive, one per line, with * and ? as wildcards (e.g. *.lab.* or Windows Server*).
    </p>
    <div class="grid gap-3 sm:grid-cols-2">
      <UiSwitch id="sync-exclude-vms" v-model="form.exclude_vms" label="Skip virtual machines" data-test="sync-exclude-vms" />
      <UiSwitch id="sync-exclude-containers" v-model="form.exclude_containers" label="Skip containers" />
      <UiSwitch id="sync-skip-stale" v-model="form.skip_stale" label="Skip stale hosts" />
      <UiSwitch id="sync-skip-retired" v-model="form.skip_retired" label="Skip retired hosts" />
      <UiTextarea id="sync-host-include" v-model="form.hostname_include" label="Only hostnames matching" hint="Empty: every hostname" :rows="3" />
      <UiTextarea id="sync-host-exclude" v-model="form.hostname_exclude" label="Skip hostnames matching" :rows="3" data-test="sync-host-exclude" />
      <UiTextarea id="sync-os-include" v-model="form.os_include" label="Only operating systems matching" hint="Empty: every OS" :rows="3" />
      <UiTextarea id="sync-os-exclude" v-model="form.os_exclude" label="Skip operating systems matching" :rows="3" />
    </div>
    <p class="mt-2 text-xs text-base-content/60">VM and container detection needs agents that report virtualization; hosts with an unknown role are imported.</p>
    <div class="mt-3 flex justify-end">
      <UiButton :loading="saving" data-test="sync-filter-save" @click="save">Save filter</UiButton>
    </div>
  </UiCard>
</template>
