import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, describe } from '@/api/client'
import type { SyncPreview, SyncResult, SyncSettings } from '@/api/types'

// Inventory sync: preview the host↔asset diff, then apply a selection.
export const useSync = defineStore('asset-sync', () => {
  const preview = ref<SyncPreview | null>(null)
  const result = ref<SyncResult | null>(null)
  const loading = ref(false)
  const error = ref('')

  async function runPreview(): Promise<void> {
    loading.value = true
    error.value = ''
    result.value = null
    try {
      preview.value = await api<SyncPreview>('POST', 'assets/inventory-sync/preview')
    } catch (e) {
      error.value = describe(e)
      preview.value = null
    } finally {
      loading.value = false
    }
  }

  async function execute(hostnames: string[]): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      result.value = await api<SyncResult>('POST', 'assets/inventory-sync/execute', { hostnames })
    } catch (e) {
      error.value = describe(e)
    } finally {
      loading.value = false
    }
  }

  // Persisted filter settings (feature 030).
  const settings = ref<SyncSettings | null>(null)
  async function loadSettings(): Promise<void> {
    settings.value = await api<SyncSettings>('GET', 'assets/inventory-sync/settings')
  }
  async function saveSettings(s: SyncSettings): Promise<void> {
    settings.value = await api<SyncSettings>('PUT', 'assets/inventory-sync/settings', {
      exclude_vms: s.exclude_vms, exclude_containers: s.exclude_containers, skip_stale: s.skip_stale, skip_retired: s.skip_retired,
      hostname_include: s.hostname_include, hostname_exclude: s.hostname_exclude, os_include: s.os_include, os_exclude: s.os_exclude,
    })
  }

  return { preview, result, loading, error, runPreview, execute, settings, loadSettings, saveSettings }
})
