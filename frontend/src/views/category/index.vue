<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, ref, onMounted } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import {
  LucideEye,
  LucideTrash,
  LucidePencil,
  LucidePlus,
} from 'shell/vben/icons';

import {
  notification,
  Space,
  Button,
  Tree,
  Card,
  Empty,
  Popconfirm,
} from 'ant-design-vue';
import type { TreeProps } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import type {
  Category,
  CategoryTreeNode,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetCategoryStore } from '../../stores/asset-category.state';

import CategoryDrawer from './category-drawer.vue';

const categoryStore = useAssetCategoryStore();

const treeData = ref<TreeProps['treeData']>([]);
const selectedCategoryId = ref<string | undefined>(undefined);
const selectedCategory = ref<Category | null>(null);
const treeLoading = ref(false);
const expandedKeys = ref<(number | string)[]>([]);

interface TreeNode {
  key: string;
  title: string;
  children?: TreeNode[];
  category: Category | undefined;
}

function buildTreeData(nodes: CategoryTreeNode[]): TreeNode[] {
  return nodes.map((node) => ({
    key: node.category?.id ?? '',
    title: node.category?.name ?? '',
    children: node.children ? buildTreeData(node.children) : undefined,
    category: node.category,
  }));
}

async function loadCategoryTree() {
  treeLoading.value = true;
  try {
    const resp = await categoryStore.getCategoryTree(undefined, 10);
    treeData.value = buildTreeData(resp.nodes ?? []);
    if (treeData.value.length > 0) {
      expandedKeys.value = treeData.value.map((n) => n.key);
    }
  } catch (e) {
    console.error('Failed to load category tree:', e);
  } finally {
    treeLoading.value = false;
  }
}

function handleTreeSelect(keys: (number | string)[], info: { node: any }) {
  if (keys.length > 0) {
    selectedCategoryId.value = String(keys[0]);
    selectedCategory.value = info.node.category ?? null;
    gridApi.reload();
  } else {
    selectedCategoryId.value = undefined;
    selectedCategory.value = null;
  }
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'query',
      label: $t('asset.page.category.name'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Category> = {
  height: 'auto',
  stripe: false,
  toolbarConfig: {
    custom: true,
    export: true,
    import: false,
    refresh: true,
    zoom: true,
  },
  exportConfig: {},
  rowConfig: {
    isHover: true,
  },
  pagerConfig: {
    enabled: true,
    pageSize: 20,
    pageSizes: [10, 20, 50, 100],
  },

  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        const resp = await categoryStore.listCategories(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            parentId: selectedCategoryId.value,
            query: formValues?.query,
          },
        );
        return {
          items: resp.items ?? [],
          total: resp.total ?? 0,
        };
      },
    },
  },

  columns: [
    { title: $t('ui.table.seq'), type: 'seq', width: 50 },
    {
      title: $t('asset.page.category.name'),
      field: 'name',
      minWidth: 150,
    },
    {
      title: $t('asset.page.category.description'),
      field: 'description',
      minWidth: 200,
    },
    {
      title: $t('asset.page.category.assetCount'),
      field: 'assetCount',
      width: 100,
    },
    {
      title: $t('asset.page.category.childCount'),
      field: 'childCount',
      width: 100,
    },
    {
      title: $t('ui.table.createdAt'),
      field: 'createTime',
      formatter: 'formatDateTime',
      width: 160,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 180,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [CategoryDrawerComponent, categoryDrawerApi] = useVbenDrawer({
  connectedComponent: CategoryDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      loadCategoryTree();
      gridApi.query();
    }
  },
});

function openDrawer(
  row: Category,
  mode: 'create' | 'edit' | 'view',
  parentId?: string,
) {
  categoryDrawerApi.setData({ row, mode, parentId });
  categoryDrawerApi.open();
}

function handleView(row: Category) {
  openDrawer(row, 'view');
}

function handleEdit(row: Category) {
  openDrawer(row, 'edit');
}

function handleCreate() {
  openDrawer({} as Category, 'create', selectedCategoryId.value);
}

function handleCreateChild(row: Category) {
  openDrawer({} as Category, 'create', row.id);
}

async function handleDelete(row: Category) {
  if (!row.id) return;
  try {
    await categoryStore.deleteCategory(row.id);
    notification.success({
      message: $t('asset.page.category.deleteSuccess'),
    });
    await loadCategoryTree();
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

function handleEditFromTree(category: Category | undefined) {
  if (!category) return;
  openDrawer(category, 'edit');
}

async function handleDeleteFromTree(category: Category | undefined) {
  if (!category?.id) return;
  try {
    await categoryStore.deleteCategory(category.id);
    notification.success({
      message: $t('asset.page.category.deleteSuccess'),
    });
    if (selectedCategoryId.value === category.id) {
      selectedCategoryId.value = undefined;
      selectedCategory.value = null;
    }
    await loadCategoryTree();
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

onMounted(() => {
  loadCategoryTree();
});
</script>

<template>
  <Page auto-content-height>
    <div class="flex h-full gap-4">
      <!-- Left: Category Tree -->
      <Card
        class="w-[300px] shrink-0"
        :title="$t('asset.page.category.tree')"
        :loading="treeLoading"
      >
        <template #extra>
          <Button type="link" size="small" @click="handleCreate">
            <template #icon>
              <component :is="LucidePlus" class="size-4" />
            </template>
          </Button>
        </template>
        <div v-if="treeData && treeData.length > 0">
          <Tree
            v-model:expandedKeys="expandedKeys"
            :tree-data="treeData"
            :selectable="true"
            :default-expand-all="true"
            @select="handleTreeSelect"
          >
            <template #title="{ title, category }">
              <div
                class="tree-node-title group flex items-center justify-between pr-1"
              >
                <span class="flex-1 truncate">{{ title }}</span>
                <span
                  class="tree-actions flex items-center"
                >
                  <Button
                    type="text"
                    size="small"
                    class="!px-1"
                    :title="$t('ui.button.edit')"
                    @click.stop="handleEditFromTree(category)"
                  >
                    <template #icon>
                      <LucidePencil class="size-3" />
                    </template>
                  </Button>
                  <Popconfirm
                    :title="$t('asset.page.category.confirmDelete')"
                    :ok-text="$t('ui.button.ok')"
                    :cancel-text="$t('ui.button.cancel')"
                    @confirm.stop="handleDeleteFromTree(category)"
                  >
                    <Button
                      type="text"
                      size="small"
                      danger
                      class="!px-1"
                      :title="$t('ui.button.delete', { moduleName: '' })"
                      @click.stop
                    >
                      <template #icon>
                        <LucideTrash class="size-3" />
                      </template>
                    </Button>
                  </Popconfirm>
                </span>
              </div>
            </template>
          </Tree>
        </div>
        <Empty v-else :description="$t('ui.table.empty')" />
      </Card>

      <!-- Right: Category List -->
      <div class="min-w-0 flex-1">
        <Grid
          :table-title="
            selectedCategory
              ? selectedCategory.name
              : $t('asset.page.category.title')
          "
        >
          <template #toolbar-tools>
            <Button class="mr-2" type="primary" @click="handleCreate">
              {{ $t('asset.page.category.create') }}
            </Button>
          </template>
          <template #action="{ row }">
            <Space>
              <Button
                type="link"
                size="small"
                :icon="h(LucideEye)"
                :title="$t('ui.button.view')"
                @click.stop="handleView(row)"
              />
              <Button
                type="link"
                size="small"
                :icon="h(LucidePlus)"
                :title="$t('asset.page.category.createChild')"
                @click.stop="handleCreateChild(row)"
              />
              <Button
                type="link"
                size="small"
                :icon="h(LucidePencil)"
                :title="$t('ui.button.edit')"
                @click.stop="handleEdit(row)"
              />
              <a-popconfirm
                :cancel-text="$t('ui.button.cancel')"
                :ok-text="$t('ui.button.ok')"
                :title="$t('asset.page.category.confirmDelete')"
                @confirm="handleDelete(row)"
              >
                <Button
                  danger
                  type="link"
                  size="small"
                  :icon="h(LucideTrash)"
                  :title="$t('ui.button.delete', { moduleName: '' })"
                />
              </a-popconfirm>
            </Space>
          </template>
        </Grid>
      </div>
    </div>

    <CategoryDrawerComponent />
  </Page>
</template>

<style scoped>
.tree-actions {
  opacity: 0;
  transition: opacity 0.2s;
}
.group:hover > .tree-actions {
  opacity: 1;
}
</style>
