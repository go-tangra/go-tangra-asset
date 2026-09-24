import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, describe } from '@/api/client'
import type { SyncPreview, SyncResult } from '@/api/types'

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

  return { preview, result, loading, error, runPreview, execute }
})
