import { defineStore } from 'pinia';

import {
  SystemService,
  type HealthCheckResponse,
  type DashboardStats,
} from '../api/services';

export const useAssetSystemStore = defineStore('asset-system', () => {
  async function healthCheck(): Promise<HealthCheckResponse> {
    return await SystemService.healthCheck();
  }

  async function getStats(): Promise<{ stats: DashboardStats }> {
    return await SystemService.getStats();
  }

  function $reset() {}

  return {
    $reset,
    healthCheck,
    getStats,
  };
});
