import { defineStore } from 'pinia';

import {
  CategoryService,
  type Category,
  type ListCategoriesResponse,
  type GetCategoryTreeResponse,
} from '../api/services';
import type { Paging } from '../api/services';

export const useAssetCategoryStore = defineStore('asset-category', () => {
  async function listCategories(
    paging?: Paging,
    formValues?: {
      query?: string;
      parentId?: string;
    } | null,
  ): Promise<ListCategoriesResponse> {
    return await CategoryService.list({
      query: formValues?.query,
      parentId: formValues?.parentId,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  async function getCategory(id: string): Promise<{ category: Category }> {
    return await CategoryService.get(id);
  }

  async function createCategory(
    data: Partial<Category>,
  ): Promise<{ category: Category }> {
    return await CategoryService.create(data);
  }

  async function updateCategory(
    id: string,
    data: Partial<Category>,
    updateMask: string[],
  ): Promise<{ category: Category }> {
    return await CategoryService.update(id, {
      id,
      data: data as Category,
      updateMask: updateMask.join(','),
    });
  }

  async function deleteCategory(id: string): Promise<void> {
    return await CategoryService.delete(id);
  }

  async function getCategoryTree(
    rootId?: string,
    maxDepth?: number,
  ): Promise<GetCategoryTreeResponse> {
    return await CategoryService.getTree({ rootId, maxDepth });
  }

  function $reset() {}

  return {
    $reset,
    listCategories,
    getCategory,
    createCategory,
    updateCategory,
    deleteCategory,
    getCategoryTree,
  };
});
