<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, computed } from 'vue';

import { Page, useVbenModal, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideEye, LucideTrash, LucidePencil } from 'shell/vben/icons';

import { notification, Space, Button, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import type { Supplier } from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetSupplierStore } from '../../stores/asset-supplier.state';

import SupplierModal from './supplier-modal.vue';

const supplierStore = useAssetSupplierStore();

const statusOptions = computed(() => [
  { value: 1, label: $t('asset.enum.supplierStatus.active') },
  { value: 2, label: $t('asset.enum.supplierStatus.inactive') },
]);

function statusToColor(status: string | number | undefined) {
  const s = Number(status);
  switch (s) {
    case 1:
      return '#52C41A';
    case 2:
      return '#8C8C8C';
    default:
      return '#C9CDD4';
  }
}

function statusToName(status: string | number | undefined) {
  const s = Number(status);
  const option = statusOptions.value.find((o) => o.value === s);
  return option?.label ?? String(status ?? '');
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
      fieldName: 'status',
      label: $t('asset.page.supplier.status'),
      componentProps: {
        options: statusOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Supplier> = {
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
        const resp = await supplierStore.listSuppliers(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            query: formValues?.query,
            status: formValues?.status,
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
      title: $t('asset.page.supplier.name'),
      field: 'name',
      minWidth: 150,
    },
    {
      title: $t('asset.page.supplier.code'),
      field: 'code',
      width: 120,
    },
    {
      title: $t('asset.page.supplier.contactPerson'),
      field: 'contactPerson',
      width: 140,
    },
    {
      title: $t('asset.page.supplier.email'),
      field: 'email',
      minWidth: 180,
    },
    {
      title: $t('asset.page.supplier.telephone'),
      field: 'telephone',
      width: 140,
    },
    {
      title: $t('asset.page.supplier.status'),
      field: 'status',
      width: 100,
      slots: { default: 'status' },
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

const [SupplierModalComponent, supplierModalApi] = useVbenModal({
  connectedComponent: SupplierModal,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function openModal(row: Supplier, mode: 'create' | 'edit' | 'view') {
  supplierModalApi.setData({ row, mode });
  supplierModalApi.open();
}

function handleView(row: Supplier) {
  openModal(row, 'view');
}

function handleEdit(row: Supplier) {
  openModal(row, 'edit');
}

function handleCreate() {
  openModal({} as Supplier, 'create');
}

async function handleDelete(row: Supplier) {
  if (!row.id) return;
  try {
    await supplierStore.deleteSupplier(row.id);
    notification.success({
      message: $t('asset.page.supplier.deleteSuccess'),
    });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('asset.page.supplier.title')">
      <template #toolbar-tools>
        <Button class="mr-2" type="primary" @click="handleCreate">
          {{ $t('asset.page.supplier.create') }}
        </Button>
      </template>
      <template #status="{ row }">
        <Tag :color="statusToColor(row.status)">
          {{ statusToName(row.status) }}
        </Tag>
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
            :title="$t('asset.page.supplier.confirmDelete')"
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

    <SupplierModalComponent />
  </Page>
</template>
