<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, ref, onMounted } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideEye, LucideTrash, LucidePencil } from 'shell/vben/icons';

import { notification, Space, Button, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import type { License, LicenseStatus } from '../../api/services';
import { SupplierService } from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetLicenseStore } from '../../stores/asset-license.state';

import LicenseDrawer from './license-drawer.vue';

const licenseStore = useAssetLicenseStore();

const supplierMap = ref<Map<string, string>>(new Map());

async function loadSuppliers() {
  try {
    const resp = await SupplierService.list({ noPaging: true });
    const map = new Map<string, string>();
    for (const sup of resp.items ?? []) {
      map.set(sup.id, sup.name);
    }
    supplierMap.value = map;
  } catch (e) {
    console.error('Failed to load suppliers:', e);
  }
}

function getSupplierName(id: string | undefined) {
  if (!id) return '-';
  return supplierMap.value.get(id) ?? id;
}

function getStatusColor(status: LicenseStatus | undefined): string {
  switch (status) {
    case 'LICENSE_STATUS_ACTIVE': return 'green';
    case 'LICENSE_STATUS_EXPIRED': return 'red';
    case 'LICENSE_STATUS_SUSPENDED': return 'orange';
    default: return 'default';
  }
}

function getStatusLabel(status: LicenseStatus | undefined): string {
  switch (status) {
    case 'LICENSE_STATUS_ACTIVE': return $t('asset.enum.licenseStatus.active');
    case 'LICENSE_STATUS_EXPIRED': return $t('asset.enum.licenseStatus.expired');
    case 'LICENSE_STATUS_SUSPENDED': return $t('asset.enum.licenseStatus.suspended');
    default: return '-';
  }
}

const statusOptions = [
  { value: 'LICENSE_STATUS_ACTIVE', label: $t('asset.enum.licenseStatus.active') },
  { value: 'LICENSE_STATUS_EXPIRED', label: $t('asset.enum.licenseStatus.expired') },
  { value: 'LICENSE_STATUS_SUSPENDED', label: $t('asset.enum.licenseStatus.suspended') },
];

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
      label: $t('asset.page.license.status'),
      componentProps: {
        options: statusOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<License> = {
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
        const resp = await licenseStore.listLicenses(
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
      title: $t('asset.page.license.name'),
      field: 'name',
      minWidth: 150,
    },
    {
      title: $t('asset.page.license.supplierId'),
      field: 'supplierId',
      width: 140,
      slots: { default: 'supplierName' },
    },
    {
      title: $t('asset.page.license.status'),
      field: 'status',
      width: 120,
      slots: { default: 'status' },
    },
    {
      title: $t('asset.page.license.validFrom'),
      field: 'validFrom',
      width: 120,
    },
    {
      title: $t('asset.page.license.validTo'),
      field: 'validTo',
      width: 120,
    },
    {
      title: $t('asset.page.license.purchaseCost'),
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

const [LicenseDrawerComponent, licenseDrawerApi] = useVbenDrawer({
  connectedComponent: LicenseDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function openDrawer(row: License, mode: 'create' | 'edit' | 'view') {
  licenseDrawerApi.setData({ row, mode });
  licenseDrawerApi.open();
}

function handleView(row: License) {
  openDrawer(row, 'view');
}

function handleEdit(row: License) {
  openDrawer(row, 'edit');
}

function handleCreate() {
  openDrawer({} as License, 'create');
}

async function handleDelete(row: License) {
  if (!row.id) return;
  try {
    await licenseStore.deleteLicense(row.id);
    notification.success({
      message: $t('asset.page.license.deleteSuccess'),
    });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

onMounted(() => {
  loadSuppliers();
});
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('asset.page.license.title')">
      <template #toolbar-tools>
        <Button class="mr-2" type="primary" @click="handleCreate">
          {{ $t('asset.page.license.create') }}
        </Button>
      </template>
      <template #supplierName="{ row }">
        {{ getSupplierName(row.supplierId) }}
      </template>
      <template #status="{ row }">
        <Tag :color="getStatusColor(row.status)">
          {{ getStatusLabel(row.status) }}
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
            :title="$t('asset.page.license.confirmDelete')"
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

    <LicenseDrawerComponent />
  </Page>
</template>
