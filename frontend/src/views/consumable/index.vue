<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, ref, onMounted } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideEye, LucideTrash, LucidePencil } from 'shell/vben/icons';

import { notification, Space, Button } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import type { Consumable } from '../../api/services';
import { CategoryService } from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetConsumableStore } from '../../stores/asset-consumable.state';

import ConsumableDrawer from './consumable-drawer.vue';

const consumableStore = useAssetConsumableStore();

const categoryMap = ref<Map<string, string>>(new Map());

async function loadCategories() {
  try {
    const resp = await CategoryService.list({ noPaging: true });
    const map = new Map<string, string>();
    for (const cat of resp.items ?? []) {
      map.set(cat.id, cat.name);
    }
    categoryMap.value = map;
  } catch (e) {
    console.error('Failed to load categories:', e);
  }
}

const categoryOptions = ref<{ value: string; label: string }[]>([]);

async function loadCategoryOptions() {
  try {
    const resp = await CategoryService.list({ noPaging: true });
    categoryOptions.value = (resp.items ?? []).map((c) => ({
      value: c.id,
      label: c.name,
    }));
  } catch (e) {
    console.error('Failed to load category options:', e);
  }
}

function getCategoryName(id: string | undefined) {
  if (!id) return '-';
  return categoryMap.value.get(id) ?? id;
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'query',
      label: $t('ui.table.search'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'categoryId',
      label: $t('asset.page.consumable.categoryId'),
      componentProps: {
        options: categoryOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Consumable> = {
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
        const resp = await consumableStore.listConsumables(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            query: formValues?.query,
            categoryId: formValues?.categoryId,
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
      title: $t('asset.page.consumable.name'),
      field: 'name',
      minWidth: 150,
    },
    {
      title: $t('asset.page.consumable.amount'),
      field: 'amount',
      width: 100,
      slots: { default: 'amount' },
    },
    {
      title: $t('asset.page.consumable.minAmount'),
      field: 'minAmount',
      width: 120,
    },
    {
      title: $t('asset.page.consumable.modelName'),
      field: 'modelName',
      width: 140,
    },
    {
      title: $t('asset.page.consumable.categoryId'),
      field: 'categoryId',
      width: 140,
      slots: { default: 'categoryName' },
    },
    {
      title: $t('asset.page.consumable.purchaseCost'),
      field: 'purchaseCost',
      width: 120,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 150,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [ConsumableDrawerComponent, consumableDrawerApi] = useVbenDrawer({
  connectedComponent: ConsumableDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function openDrawer(row: Consumable, mode: 'create' | 'edit' | 'view') {
  consumableDrawerApi.setData({ row, mode });
  consumableDrawerApi.open();
}

function handleView(row: Consumable) {
  openDrawer(row, 'view');
}

function handleEdit(row: Consumable) {
  openDrawer(row, 'edit');
}

function handleCreate() {
  openDrawer({} as Consumable, 'create');
}

async function handleDelete(row: Consumable) {
  if (!row.id) return;
  try {
    await consumableStore.deleteConsumable(row.id);
    notification.success({
      message: $t('asset.page.consumable.deleteSuccess'),
    });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

onMounted(() => {
  loadCategories();
  loadCategoryOptions();
});
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('asset.page.consumable.title')">
      <template #toolbar-tools>
        <Button class="mr-2" type="primary" @click="handleCreate">
          {{ $t('asset.page.consumable.create') }}
        </Button>
      </template>
      <template #amount="{ row }">
        <span
          :style="{
            color:
              row.minAmount && row.amount < row.minAmount
                ? '#FF4D4F'
                : undefined,
            fontWeight:
              row.minAmount && row.amount < row.minAmount
                ? 'bold'
                : undefined,
          }"
        >
          {{ row.amount ?? 0 }}
          <span
            v-if="row.minAmount && row.amount < row.minAmount"
            style="font-size: 11px"
          >
            ({{ $t('asset.page.consumable.lowStock') }})
          </span>
        </span>
      </template>
      <template #categoryName="{ row }">
        {{ getCategoryName(row.categoryId) }}
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
            :icon="h(LucidePencil)"
            :title="$t('ui.button.edit')"
            @click.stop="handleEdit(row)"
          />
          <a-popconfirm
            :cancel-text="$t('ui.button.cancel')"
            :ok-text="$t('ui.button.ok')"
            :title="$t('asset.page.consumable.confirmDelete')"
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

    <ConsumableDrawerComponent />
  </Page>
</template>
