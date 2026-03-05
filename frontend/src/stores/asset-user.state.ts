import { defineStore } from 'pinia';

import {
  UserService,
  type ListUsersResponse,
} from '../api/services';

export const useAssetUserStore = defineStore('asset-user', () => {
  async function listUsers(): Promise<ListUsersResponse> {
    return await UserService.list();
  }

  function $reset() {}

  return {
    $reset,
    listUsers,
  };
});
