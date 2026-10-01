import { defineStore } from 'pinia'
import { api } from '@/api/client'
import type { Consumable, InsurancePolicy, License, ListParams, Page, PolicyAsset } from '@/api/types'
import { listSpec, pagedList } from './paged'

// Consumables, licenses and insurance policies (+ per-asset coverage). Each
// table is one server page (list contract); the CRUD calls live in
// useInventories.

/** Text search of the simple lists. */
export interface SearchFilter {
  query?: string | undefined
}

/** Sortable fields of GET /consumables (server Spec store.ConsumableList). */
export const CONSUMABLE_LIST = listSpec(['name', 'amount', 'created_at'], 'name')
/** Sortable fields of GET /licenses (store.LicenseList). */
export const LICENSE_LIST = listSpec(['name', 'valid_to', 'created_at'], 'name')
/** Sortable fields of GET /insurance-policies (store.InsuranceList). */
export const POLICY_LIST = listSpec(['name', 'valid_to', 'created_at'], 'name')
/** Sortable fields of GET /insurance-policies/{id}/assets (store.PolicyAssetList). */
export const COVERED_LIST = listSpec(['asset_tag', 'name'], 'asset_tag')

export const useConsumableList = defineStore('asset-consumable-list', () => pagedList<Consumable, SearchFilter>('consumables', CONSUMABLE_LIST.first))
export const useLicenseList = defineStore('asset-license-list', () => pagedList<License, SearchFilter>('licenses', LICENSE_LIST.first))
export const usePolicyList = defineStore('asset-policy-list', () => pagedList<InsurancePolicy, SearchFilter>('insurance-policies', POLICY_LIST.first))

export const useInventories = defineStore('asset-inventories', () => {
  const createConsumable = (input: Partial<Consumable>) => api<Consumable>('POST', 'consumables', input)
  const updateConsumable = (id: string, input: Partial<Consumable>) => api<Consumable>('PUT', 'consumables/' + id, input)
  const removeConsumable = (id: string) => api<void>('DELETE', 'consumables/' + id)

  const createLicense = (input: Partial<License>) => api<License>('POST', 'licenses', input)
  const updateLicense = (id: string, input: Partial<License>) => api<License>('PUT', 'licenses/' + id, input)
  const removeLicense = (id: string) => api<void>('DELETE', 'licenses/' + id)

  const createPolicy = (input: Partial<InsurancePolicy>) => api<InsurancePolicy>('POST', 'insurance-policies', input)
  const updatePolicy = (id: string, input: Partial<InsurancePolicy>) => api<InsurancePolicy>('PUT', 'insurance-policies/' + id, input)
  const removePolicy = (id: string) => api<void>('DELETE', 'insurance-policies/' + id)
  /** One page of the assets a policy covers. */
  const policyAssets = (id: string, q: ListParams = COVERED_LIST.first) =>
    api<Page<PolicyAsset>>('GET', 'insurance-policies/' + id + '/assets', undefined, { query: { ...q } })
  const addPolicyAsset = (id: string, asset_id: string, covered_value: number, notes = '') =>
    api<PolicyAsset>('POST', 'insurance-policies/' + id + '/assets', { asset_id, covered_value, notes })
  const removePolicyAsset = (id: string, assetId: string) => api<void>('DELETE', 'insurance-policies/' + id + '/assets/' + assetId)

  return {
    createConsumable, updateConsumable, removeConsumable,
    createLicense, updateLicense, removeLicense,
    createPolicy, updatePolicy, removePolicy, policyAssets, addPolicyAsset, removePolicyAsset,
  }
})
