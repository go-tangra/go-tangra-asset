import { defineStore } from 'pinia';

import {
  LocationService,
  type Location,
  type LocationStatus,
  type ListLocationsResponse,
  type GetLocationTreeResponse,
} from '../api/services';
import type { Paging } from '../api/services';

export const useAssetLocationStore = defineStore('asset-location', () => {
  async function listLocations(
    paging?: Paging,
    formValues?: {
      query?: string;
      parentId?: string;
      status?: LocationStatus;
    } | null,
  ): Promise<ListLocationsResponse> {
    return await LocationService.list({
      query: formValues?.query,
      parentId: formValues?.parentId,
      status: formValues?.status,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  async function getLocation(id: string): Promise<{ location: Location }> {
    return await LocationService.get(id);
  }

  async function createLocation(
    data: Partial<Location>,
  ): Promise<{ location: Location }> {
    return await LocationService.create(data);
  }

  async function updateLocation(
    id: string,
    data: Partial<Location>,
    updateMask: string[],
  ): Promise<{ location: Location }> {
    return await LocationService.update(id, {
      id,
      data: data as Location,
      updateMask: updateMask.join(','),
    });
  }

  async function deleteLocation(id: string): Promise<void> {
    return await LocationService.delete(id);
  }

  async function getLocationTree(
    rootId?: string,
    maxDepth?: number,
  ): Promise<GetLocationTreeResponse> {
    return await LocationService.getTree({ rootId, maxDepth });
  }

  function $reset() {}

  return {
    $reset,
    listLocations,
    getLocation,
    createLocation,
    updateLocation,
    deleteLocation,
    getLocationTree,
  };
});
