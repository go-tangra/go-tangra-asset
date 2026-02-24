import { defineStore } from 'pinia';

import {
  AssetService,
  type Asset,
  type AssetStatus,
  type AssetDocument,
  type ListAssetsResponse,
  type ListAssignmentsResponse,
  type ListDocumentsResponse,
} from '../api/services';
import type { Paging } from '../api/services';

export const useAssetAssetStore = defineStore('asset-asset', () => {
  async function listAssets(
    paging?: Paging,
    formValues?: {
      query?: string;
      status?: AssetStatus;
      categoryId?: string;
      supplierId?: string;
      locationId?: string;
      employeeId?: string;
    } | null,
  ): Promise<ListAssetsResponse> {
    return await AssetService.list({
      query: formValues?.query,
      status: formValues?.status,
      categoryId: formValues?.categoryId,
      supplierId: formValues?.supplierId,
      locationId: formValues?.locationId,
      employeeId: formValues?.employeeId,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  async function getAsset(id: string): Promise<{ asset: Asset }> {
    return await AssetService.get(id);
  }

  async function createAsset(
    data: Partial<Asset>,
  ): Promise<{ asset: Asset }> {
    return await AssetService.create(data);
  }

  async function updateAsset(
    id: string,
    data: Partial<Asset>,
    updateMask: string[],
  ): Promise<{ asset: Asset }> {
    return await AssetService.update(id, {
      id,
      data: data as Asset,
      updateMask: updateMask.join(','),
    });
  }

  async function deleteAsset(id: string): Promise<void> {
    return await AssetService.delete(id);
  }

  async function assignAsset(
    id: string,
    employeeId: string,
    notes?: string,
  ): Promise<{ asset: Asset }> {
    return await AssetService.assign(id, { employeeId, notes });
  }

  async function unassignAsset(
    id: string,
    locationId?: string,
    notes?: string,
  ): Promise<{ asset: Asset }> {
    return await AssetService.unassign(id, { locationId, notes });
  }

  async function getAssignmentHistory(
    id: string,
  ): Promise<ListAssignmentsResponse> {
    return await AssetService.getAssignmentHistory(id);
  }

  async function uploadPhoto(
    id: string,
    content: string,
    fileName: string,
  ): Promise<{ asset: Asset }> {
    return await AssetService.uploadPhoto(id, { content, fileName });
  }

  async function deletePhoto(id: string): Promise<void> {
    return await AssetService.deletePhoto(id);
  }

  async function listDocuments(id: string): Promise<ListDocumentsResponse> {
    return await AssetService.listDocuments(id);
  }

  async function uploadDocument(
    id: string,
    fileName: string,
    content: string,
    description?: string,
  ): Promise<{ document: AssetDocument }> {
    return await AssetService.uploadDocument(id, {
      fileName,
      content,
      description,
    });
  }

  async function deleteDocument(
    assetId: string,
    documentId: string,
  ): Promise<void> {
    return await AssetService.deleteDocument(assetId, documentId);
  }

  async function downloadDocument(
    assetId: string,
    documentId: string,
  ): Promise<{ content: string; fileName: string; mimeType: string }> {
    return await AssetService.downloadDocument(assetId, documentId);
  }

  function $reset() {}

  return {
    $reset,
    listAssets,
    getAsset,
    createAsset,
    updateAsset,
    deleteAsset,
    assignAsset,
    unassignAsset,
    getAssignmentHistory,
    uploadPhoto,
    deletePhoto,
    listDocuments,
    uploadDocument,
    deleteDocument,
    downloadDocument,
  };
});
