import { defineStore } from 'pinia';

import {
  LicenseService,
  type License,
  type AssetDocument,
  type ListLicensesResponse,
  type ListDocumentsResponse,
  type LicenseStatus,
} from '../api/services';
import type { Paging } from '../api/services';

export const useAssetLicenseStore = defineStore('asset-license', () => {
  async function listLicenses(
    paging?: Paging,
    formValues?: {
      query?: string;
      supplierId?: string;
      status?: LicenseStatus;
    } | null,
  ): Promise<ListLicensesResponse> {
    return await LicenseService.list({
      query: formValues?.query,
      supplierId: formValues?.supplierId,
      status: formValues?.status,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  async function getLicense(id: string): Promise<{ license: License }> {
    return await LicenseService.get(id);
  }

  async function createLicense(
    data: Partial<License>,
  ): Promise<{ license: License }> {
    return await LicenseService.create(data);
  }

  async function updateLicense(
    id: string,
    data: Partial<License>,
    updateMask: string[],
  ): Promise<{ license: License }> {
    return await LicenseService.update(id, {
      id,
      data: data as License,
      updateMask: updateMask.join(','),
    });
  }

  async function deleteLicense(id: string): Promise<void> {
    return await LicenseService.delete(id);
  }

  async function listDocuments(id: string): Promise<ListDocumentsResponse> {
    return await LicenseService.listDocuments(id);
  }

  async function uploadDocument(
    id: string,
    fileName: string,
    content: string,
    description?: string,
  ): Promise<{ document: AssetDocument }> {
    return await LicenseService.uploadDocument(id, {
      fileName,
      content,
      description,
    });
  }

  async function deleteDocument(
    licenseId: string,
    documentId: string,
  ): Promise<void> {
    return await LicenseService.deleteDocument(licenseId, documentId);
  }

  async function downloadDocument(
    licenseId: string,
    documentId: string,
  ): Promise<{ content: string; fileName: string; mimeType: string }> {
    return await LicenseService.downloadDocument(licenseId, documentId);
  }

  function $reset() {}

  return {
    $reset,
    listLicenses,
    getLicense,
    createLicense,
    updateLicense,
    deleteLicense,
    listDocuments,
    uploadDocument,
    deleteDocument,
    downloadDocument,
  };
});
