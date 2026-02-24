import { defineStore } from 'pinia';

import {
  SupplierService,
  type Supplier,
  type ListSuppliersResponse,
} from '../api/services';
import type { Paging } from '../api/services';

export const useAssetSupplierStore = defineStore('asset-supplier', () => {
  async function listSuppliers(
    paging?: Paging,
    formValues?: {
      query?: string;
      status?: string;
    } | null,
  ): Promise<ListSuppliersResponse> {
    return await SupplierService.list({
      query: formValues?.query,
      status: formValues?.status,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  async function getSupplier(id: string): Promise<{ supplier: Supplier }> {
    return await SupplierService.get(id);
  }

  async function createSupplier(
    data: Partial<Supplier>,
  ): Promise<{ supplier: Supplier }> {
    return await SupplierService.create(data);
  }

  async function updateSupplier(
    id: string,
    data: Partial<Supplier>,
    updateMask: string[],
  ): Promise<{ supplier: Supplier }> {
    return await SupplierService.update(id, {
      id,
      data: data as Supplier,
      updateMask: updateMask.join(','),
    });
  }

  async function deleteSupplier(id: string): Promise<void> {
    return await SupplierService.delete(id);
  }

  function $reset() {}

  return {
    $reset,
    listSuppliers,
    getSupplier,
    createSupplier,
    updateSupplier,
    deleteSupplier,
  };
});
