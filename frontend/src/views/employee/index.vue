<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h } from 'vue';

import { Page, useVbenModal, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideEye, LucideTrash, LucidePencil, LucideRefreshCw } from 'shell/vben/icons';

import { notification, Space, Button } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import type { Employee } from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetEmployeeStore } from '../../stores/asset-employee.state';

import EmployeeModal from './employee-modal.vue';
import LdapSyncModal from './ldap-sync-modal.vue';

const employeeStore = useAssetEmployeeStore();

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
      component: 'Input',
      fieldName: 'department',
      label: $t('asset.page.employee.department'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Employee> = {
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
        const resp = await employeeStore.listEmployees(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            query: formValues?.query,
            department: formValues?.department,
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
      title: $t('asset.page.employee.fullName'),
      field: 'firstName',
      minWidth: 150,
      slots: { default: 'fullName' },
    },
    {
      title: $t('asset.page.employee.employeeNumber'),
      field: 'employeeNumber',
      width: 140,
    },
    {
      title: $t('asset.page.employee.email'),
      field: 'email',
      minWidth: 180,
    },
    {
      title: $t('asset.page.employee.department'),
      field: 'department',
      width: 140,
    },
    {
      title: $t('asset.page.employee.jobTitle'),
      field: 'jobTitle',
      width: 140,
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

const [EmployeeModalComponent, employeeModalApi] = useVbenModal({
  connectedComponent: EmployeeModal,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

const [LdapSyncModalComponent, ldapSyncModalApi] = useVbenModal({
  connectedComponent: LdapSyncModal,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function handleLdapSync() {
  ldapSyncModalApi.open();
}

function openModal(row: Employee, mode: 'create' | 'edit' | 'view') {
  employeeModalApi.setData({ row, mode });
  employeeModalApi.open();
}

function handleView(row: Employee) {
  openModal(row, 'view');
}

function handleEdit(row: Employee) {
  openModal(row, 'edit');
}

function handleCreate() {
  openModal({} as Employee, 'create');
}

async function handleDelete(row: Employee) {
  if (!row.id) return;
  try {
    await employeeStore.deleteEmployee(row.id);
    notification.success({
      message: $t('asset.page.employee.deleteSuccess'),
    });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('asset.page.employee.title')">
      <template #toolbar-tools>
        <Button class="mr-2" @click="handleLdapSync">
          <template #icon><LucideRefreshCw class="size-4" /></template>
          {{ $t('asset.page.employee.ldap.syncButton') }}
        </Button>
        <Button class="mr-2" type="primary" @click="handleCreate">
          {{ $t('asset.page.employee.create') }}
        </Button>
      </template>
      <template #fullName="{ row }">
        <span>{{ row.firstName }} {{ row.lastName }}</span>
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
            :title="$t('asset.page.employee.confirmDelete')"
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

    <EmployeeModalComponent />
    <LdapSyncModalComponent />
  </Page>
</template>
