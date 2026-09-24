import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, describe } from '@/api/client'
import type { Asset, Assignment, User } from '@/api/types'
import type { AssetInput } from '@/schemas/asset'

export interface AssetFilter {
  query?: string | undefined
  status?: string | undefined
  category_id?: string | undefined
  supplier_id?: string | undefined
  location_id?: string | undefined
  user_id?: string | undefined
  cursor?: string | undefined
  limit?: number | undefined
}

export const useAssets = defineStore('asset-assets', () => {
  const items = ref<Asset[]>([])
  const users = ref<User[]>([])
  const loading = ref(false)
  const error = ref('')

  async function list(filter: AssetFilter = {}): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await api<{ items: Asset[] }>('GET', 'assets', undefined, { query: { ...filter } })
      items.value = res.items ?? []
    } catch (e) {
      error.value = describe(e)
    } finally {
      loading.value = false
    }
  }
  const get = (id: string) => api<Asset>('GET', 'assets/' + id)
  const create = (input: AssetInput) => api<Asset>('POST', 'assets', input)
  const update = (id: string, input: AssetInput) => api<Asset>('PUT', 'assets/' + id, input)
  const remove = (id: string) => api<void>('DELETE', 'assets/' + id)
  const assign = (id: string, user_id: string, notes = '') => api<Asset>('POST', 'assets/' + id + '/assign', { user_id, notes })
  const unassign = (id: string, location_id = '', notes = '') => api<Asset>('POST', 'assets/' + id + '/unassign', { location_id, notes })
  const history = async (id: string): Promise<Assignment[]> => (await api<{ items: Assignment[] }>('GET', 'assets/' + id + '/assignments')).items ?? []
  const deletePhoto = (id: string) => api<void>('DELETE', 'assets/' + id + '/photo')

  async function loadUsers(query = ''): Promise<void> {
    try {
      users.value = (await api<{ items: User[] }>('GET', 'users', undefined, { query: { query } })).items ?? []
    } catch {
      users.value = []
    }
  }

  /** Patches an asset in the loaded list (live events). */
  function patch(a: Asset): void {
    const i = items.value.findIndex((x) => x.id === a.id)
    if (i >= 0) items.value[i] = a
  }

  return { items, users, loading, error, list, get, create, update, remove, assign, unassign, history, deletePhoto, loadUsers, patch }
})
