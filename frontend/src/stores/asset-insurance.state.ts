import { defineStore } from 'pinia';

import {
  InsurancePolicyService,
  type InsurancePolicy,
  type InsurancePolicyStatus,
  type CoverageType,
  type ListInsurancePoliciesResponse,
  type ListPolicyAssetsResponse,
  type PolicyAsset,
} from '../api/services';
import type { Paging } from '../api/services';

export const useAssetInsuranceStore = defineStore('asset-insurance', () => {
  async function listPolicies(
    paging?: Paging,
    formValues?: {
      query?: string;
      status?: InsurancePolicyStatus;
      coverageType?: CoverageType;
    } | null,
  ): Promise<ListInsurancePoliciesResponse> {
    return await InsurancePolicyService.list({
      query: formValues?.query,
      status: formValues?.status,
      coverageType: formValues?.coverageType,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  async function getPolicy(
    id: string,
  ): Promise<{ insurancePolicy: InsurancePolicy }> {
    return await InsurancePolicyService.get(id);
  }

  async function createPolicy(
    data: Partial<InsurancePolicy>,
  ): Promise<{ insurancePolicy: InsurancePolicy }> {
    return await InsurancePolicyService.create(data);
  }

  async function updatePolicy(
    id: string,
    data: Partial<InsurancePolicy>,
    updateMask: string[],
  ): Promise<{ insurancePolicy: InsurancePolicy }> {
    return await InsurancePolicyService.update(id, {
      id,
      data: data as InsurancePolicy,
      updateMask: updateMask.join(','),
    });
  }

  async function deletePolicy(id: string): Promise<void> {
    return await InsurancePolicyService.delete(id);
  }

  async function listPolicyAssets(
    id: string,
  ): Promise<ListPolicyAssetsResponse> {
    return await InsurancePolicyService.listAssets(id);
  }

  async function addAssetToPolicy(
    id: string,
    assetId: string,
    coveredValue?: number,
    notes?: string,
  ): Promise<{ policyAsset: PolicyAsset }> {
    return await InsurancePolicyService.addAsset(id, {
      assetId,
      coveredValue,
      notes,
    });
  }

  async function removeAssetFromPolicy(
    id: string,
    assetId: string,
  ): Promise<void> {
    return await InsurancePolicyService.removeAsset(id, assetId);
  }

  function $reset() {}

  return {
    $reset,
    listPolicies,
    getPolicy,
    createPolicy,
    updatePolicy,
    deletePolicy,
    listPolicyAssets,
    addAssetToPolicy,
    removeAssetFromPolicy,
  };
});
