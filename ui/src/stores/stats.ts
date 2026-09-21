import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, describe } from '@/api/client'
import type { Dashboard } from '@/api/types'

export const useStats = defineStore('asset-stats', () => {
  const snapshot = ref<Dashboard | null>(null)
  const error = ref('')

  async function load(): Promise<void> {
    error.value = ''
    try {
      snapshot.value = await api<Dashboard>('GET', 'stats')
    } catch (e) {
      error.value = describe(e)
      snapshot.value = null
    }
  }

  const exportBackup = () => api<Record<string, unknown>>('POST', 'backup/export')
  const importBackup = (backup: unknown, mode: 'skip' | 'overwrite', full = false) =>
    api<{ imported: Record<string, number>; skipped: Record<string, number>; deleted: number }>('POST', 'backup/import', { mode, full, backup })

  return { snapshot, error, load, exportBackup, importBackup }
})
