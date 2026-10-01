import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, describe } from '@/api/client'
import type { Category, Location, Supplier } from '@/api/types'
import { listSpec, loadOptions, pagedList } from './paged'

/** Sortable fields of GET /suppliers (server Spec store.SupplierList). */
export const SUPPLIER_LIST = listSpec(['name', 'created_at'], 'name')

/** The suppliers table: one server page (list contract). */
export const useSupplierList = defineStore('asset-supplier-list', () => pagedList<Supplier, { query?: string | undefined }>('suppliers', SUPPLIER_LIST.first))

// Categories, suppliers and locations: the classification records assets refer to.
export const useOrg = defineStore('asset-org', () => {
  const categories = ref<Category[]>([])
  const categoryTree = ref<Category[]>([])
  const suppliers = ref<Supplier[]>([])
  const locations = ref<Location[]>([])
  const locationTree = ref<Location[]>([])
  const error = ref('')

  async function guard<T>(fn: () => Promise<T>): Promise<T | undefined> {
    error.value = ''
    try {
      return await fn()
    } catch (e) {
      error.value = describe(e)
      return undefined
    }
  }

  const loadCategories = () =>
    guard(async () => {
      categories.value = (await api<{ items: Category[] }>('GET', 'categories')).items ?? []
      categoryTree.value = (await api<{ items: Category[] }>('GET', 'categories/tree')).items ?? []
    })
  const createCategory = (input: Partial<Category>) => api<Category>('POST', 'categories', input)
  const updateCategory = (id: string, input: Partial<Category>) => api<Category>('PUT', 'categories/' + id, input)
  const removeCategory = (id: string) => api<void>('DELETE', 'categories/' + id)

  /** Suppliers for selects and name lookups (by name, up to the largest page). */
  const loadSuppliers = () =>
    guard(async () => {
      suppliers.value = await loadOptions<Supplier>('suppliers', 'name')
    })
  const createSupplier = (input: Partial<Supplier>) => api<Supplier>('POST', 'suppliers', input)
  const updateSupplier = (id: string, input: Partial<Supplier>) => api<Supplier>('PUT', 'suppliers/' + id, input)
  const removeSupplier = (id: string) => api<void>('DELETE', 'suppliers/' + id)

  const loadLocations = () =>
    guard(async () => {
      locations.value = (await api<{ items: Location[] }>('GET', 'locations')).items ?? []
      locationTree.value = (await api<{ items: Location[] }>('GET', 'locations/tree')).items ?? []
    })
  const createLocation = (input: Partial<Location>) => api<Location>('POST', 'locations', input)
  const updateLocation = (id: string, input: Partial<Location>) => api<Location>('PUT', 'locations/' + id, input)
  const removeLocation = (id: string) => api<void>('DELETE', 'locations/' + id)

  const categoryName = (id?: string) => categories.value.find((c) => c.id === id)?.name ?? ''
  const supplierName = (id?: string) => suppliers.value.find((s) => s.id === id)?.name ?? ''
  const locationName = (id?: string) => locations.value.find((l) => l.id === id)?.path ?? locations.value.find((l) => l.id === id)?.name ?? ''

  return {
    categories, categoryTree, suppliers, locations, locationTree, error,
    loadCategories, createCategory, updateCategory, removeCategory,
    loadSuppliers, createSupplier, updateSupplier, removeSupplier,
    loadLocations, createLocation, updateLocation, removeLocation,
    categoryName, supplierName, locationName,
  }
})
