import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, describe } from '@/api/client'
import type { Consumable, InsurancePolicy, License, PolicyAsset } from '@/api/types'

// Consumables, licenses and insurance policies (+ per-asset coverage).
export const useInventories = defineStore('asset-inventories', () => {
  const consumables = ref<Consumable[]>([])
  const licenses = ref<License[]>([])
  const policies = ref<InsurancePolicy[]>([])
  const error = ref('')

  async function guard(fn: () => Promise<void>): Promise<void> {
    error.value = ''
    try {
      await fn()
    } catch (e) {
      error.value = describe(e)
    }
  }

  const loadConsumables = (query = '') =>
    guard(async () => {
      consumables.value = (await api<{ items: Consumable[] }>('GET', 'consumables', undefined, { query: { query } })).items ?? []
    })
  const createConsumable = (input: Partial<Consumable>) => api<Consumable>('POST', 'consumables', input)
  const updateConsumable = (id: string, input: Partial<Consumable>) => api<Consumable>('PUT', 'consumables/' + id, input)
  const removeConsumable = (id: string) => api<void>('DELETE', 'consumables/' + id)

  const loadLicenses = (query = '') =>
    guard(async () => {
      licenses.value = (await api<{ items: License[] }>('GET', 'licenses', undefined, { query: { query } })).items ?? []
    })
  const createLicense = (input: Partial<License>) => api<License>('POST', 'licenses', input)
  const updateLicense = (id: string, input: Partial<License>) => api<License>('PUT', 'licenses/' + id, input)
  const removeLicense = (id: string) => api<void>('DELETE', 'licenses/' + id)

  const loadPolicies = (query = '') =>
    guard(async () => {
      policies.value = (await api<{ items: InsurancePolicy[] }>('GET', 'insurance-policies', undefined, { query: { query } })).items ?? []
    })
  const createPolicy = (input: Partial<InsurancePolicy>) => api<InsurancePolicy>('POST', 'insurance-policies', input)
  const updatePolicy = (id: string, input: Partial<InsurancePolicy>) => api<InsurancePolicy>('PUT', 'insurance-policies/' + id, input)
  const removePolicy = (id: string) => api<void>('DELETE', 'insurance-policies/' + id)
  const policyAssets = async (id: string): Promise<PolicyAsset[]> => (await api<{ items: PolicyAsset[] }>('GET', 'insurance-policies/' + id + '/assets')).items ?? []
  const addPolicyAsset = (id: string, asset_id: string, covered_value: number, notes = '') =>
    api<PolicyAsset>('POST', 'insurance-policies/' + id + '/assets', { asset_id, covered_value, notes })
  const removePolicyAsset = (id: string, assetId: string) => api<void>('DELETE', 'insurance-policies/' + id + '/assets/' + assetId)

  return {
    consumables, licenses, policies, error,
    loadConsumables, createConsumable, updateConsumable, removeConsumable,
    loadLicenses, createLicense, updateLicense, removeLicense,
    loadPolicies, createPolicy, updatePolicy, removePolicy, policyAssets, addPolicyAsset, removePolicyAsset,
  }
})
