import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/client'
import type { Asset, Assignment, ListParams, Page, User } from '@/api/types'
import type { AssetInput } from '@/schemas/asset'
import { listSpec, pagedList, PAGE_SIZE } from './paged'

/** Sortable fields of GET /assets (server Spec store.AssetList). */
export const ASSET_SORTS = ['asset_tag', 'name', 'status', 'category', 'location', 'purchase_date', 'warranty_end', 'created_at'] as const
export const ASSET_LIST = listSpec(ASSET_SORTS, 'asset_tag')
/** Sortable fields of GET /assets/{id}/assignments (store.AssignmentList): newest first. */
export const ASSIGNMENT_LIST = listSpec(['assigned_at', 'returned_at'], 'assigned_at', 'desc')

export interface AssetFilter {
  query?: string | undefined
  status?: string | undefined
  category_id?: string | undefined
  supplier_id?: string | undefined
  location_id?: string | undefined
  user_id?: string | undefined
}

export const useAssets = defineStore('asset-assets', () => {
  // The assets table: one server page (GET /assets, list contract).
  const page = pagedList<Asset, AssetFilter>('assets', ASSET_LIST.first)
  const users = ref<User[]>([])
  /** Assets for a picker (insurance coverage): the first matches of a search. */
  const options = ref<Asset[]>([])
  const get = (id: string) => api<Asset>('GET', 'assets/' + id)
  const create = (input: AssetInput) => api<Asset>('POST', 'assets', input)
  const update = (id: string, input: AssetInput) => api<Asset>('PUT', 'assets/' + id, input)
  const remove = (id: string) => api<void>('DELETE', 'assets/' + id)
  const assign = (id: string, user_id: string, notes = '') => api<Asset>('POST', 'assets/' + id + '/assign', { user_id, notes })
  const unassign = (id: string, location_id = '', notes = '') => api<Asset>('POST', 'assets/' + id + '/unassign', { location_id, notes })
  /** One page of an asset's assignment history (newest first by default). */
  const history = (id: string, q: ListParams = ASSIGNMENT_LIST.first) => api<Page<Assignment>>('GET', 'assets/' + id + '/assignments', undefined, { query: { ...q } })
  const deletePhoto = (id: string) => api<void>('DELETE', 'assets/' + id + '/photo')

  async function loadUsers(query = ''): Promise<void> {
    try {
      users.value = (await api<{ items: User[] }>('GET', 'users', undefined, { query: { query } })).items ?? []
    } catch {
      users.value = []
    }
  }

  let optSeq = 0
  /** Loads the first assets matching query (by tag) into options; superseded searches are ignored. */
  async function searchOptions(query = ''): Promise<void> {
    const mine = ++optSeq
    const q = query.trim()
    let rows: Asset[] = []
    try {
      rows = (await api<Page<Asset>>('GET', 'assets', undefined, { query: { ...(q ? { query: q } : {}), page: 1, page_size: PAGE_SIZE * 2, sort: 'asset_tag', order: 'asc' } })).items ?? []
    } catch {
      rows = []
    }
    if (mine === optSeq) options.value = rows
  }

  /** Patches an asset in the loaded page (live events). */
  function patch(a: Asset): void {
    const i = page.items.value.findIndex((x) => x.id === a.id)
    if (i >= 0) page.items.value[i] = a
  }

  return { ...page, users, options, get, create, update, remove, assign, unassign, history, deletePhoto, loadUsers, searchOptions, patch }
})
