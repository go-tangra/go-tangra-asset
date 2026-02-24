import { defineStore } from 'pinia';

import {
  ConsumableService,
  type Consumable,
  type AssetDocument,
  type ListConsumablesResponse,
  type ListDocumentsResponse,
} from '../api/services';
import type { Paging } from '../api/services';

export const useAssetConsumableStore = defineStore('asset-consumable', () => {
  async function listConsumables(
    paging?: Paging,
    formValues?: {
      query?: string;
      categoryId?: string;
      supplierId?: string;
      locationId?: string;
    } | null,
  ): Promise<ListConsumablesResponse> {
    return await ConsumableService.list({
      query: formValues?.query,
      categoryId: formValues?.categoryId,
      supplierId: formValues?.supplierId,
      locationId: formValues?.locationId,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  async function getConsumable(
    id: string,
  ): Promise<{ consumable: Consumable }> {
    return await ConsumableService.get(id);
  }

  async function createConsumable(
    data: Partial<Consumable>,
  ): Promise<{ consumable: Consumable }> {
    return await ConsumableService.create(data);
  }

  async function updateConsumable(
    id: string,
    data: Partial<Consumable>,
    updateMask: string[],
  ): Promise<{ consumable: Consumable }> {
    return await ConsumableService.update(id, {
      id,
      data: data as Consumable,
      updateMask: updateMask.join(','),
    });
  }

  async function deleteConsumable(id: string): Promise<void> {
    return await ConsumableService.delete(id);
  }

  async function listDocuments(id: string): Promise<ListDocumentsResponse> {
    return await ConsumableService.listDocuments(id);
  }

  async function uploadDocument(
    id: string,
    fileName: string,
    content: string,
    description?: string,
  ): Promise<{ document: AssetDocument }> {
    return await ConsumableService.uploadDocument(id, {
      fileName,
      content,
      description,
    });
  }

  async function deleteDocument(
    consumableId: string,
    documentId: string,
  ): Promise<void> {
    return await ConsumableService.deleteDocument(consumableId, documentId);
  }

  async function downloadDocument(
    consumableId: string,
    documentId: string,
  ): Promise<{ content: string; fileName: string; mimeType: string }> {
    return await ConsumableService.downloadDocument(consumableId, documentId);
  }

  function $reset() {}

  return {
    $reset,
    listConsumables,
    getConsumable,
    createConsumable,
    updateConsumable,
    deleteConsumable,
    listDocuments,
    uploadDocument,
    deleteDocument,
    downloadDocument,
  };
});
