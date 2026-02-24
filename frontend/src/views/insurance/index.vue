<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, ref, onMounted } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import { LucideEye, LucideTrash, LucidePencil } from 'shell/vben/icons';

import { notification, Space, Button, Tag } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import type {
  InsurancePolicy,
  InsurancePolicyStatus,
  CoverageType,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetInsuranceStore } from '../../stores/asset-insurance.state';

import InsuranceDrawer from './insurance-drawer.vue';

const insuranceStore = useAssetInsuranceStore();

function getStatusColor(status: InsurancePolicyStatus | undefined): string {
  switch (status) {
    case 'INSURANCE_POLICY_STATUS_ACTIVE':
      return 'green';
    case 'INSURANCE_POLICY_STATUS_EXPIRED':
      return 'red';
    case 'INSURANCE_POLICY_STATUS_CANCELLED':
      return 'orange';
    default:
      return 'default';
  }
}

function getStatusLabel(status: InsurancePolicyStatus | undefined): string {
  switch (status) {
    case 'INSURANCE_POLICY_STATUS_ACTIVE':
      return $t('asset.enum.insurancePolicyStatus.active');
    case 'INSURANCE_POLICY_STATUS_EXPIRED':
      return $t('asset.enum.insurancePolicyStatus.expired');
    case 'INSURANCE_POLICY_STATUS_CANCELLED':
      return $t('asset.enum.insurancePolicyStatus.cancelled');
    default:
      return '-';
  }
}

function getCoverageTypeLabel(ct: CoverageType | undefined): string {
  switch (ct) {
    case 'COVERAGE_TYPE_ALL_RISK':
      return $t('asset.enum.coverageType.allRisk');
    case 'COVERAGE_TYPE_FIRE_THEFT':
      return $t('asset.enum.coverageType.fireTheft');
    case 'COVERAGE_TYPE_LIABILITY':
      return $t('asset.enum.coverageType.liability');
    case 'COVERAGE_TYPE_EQUIPMENT_BREAKDOWN':
      return $t('asset.enum.coverageType.equipmentBreakdown');
    case 'COVERAGE_TYPE_CYBER':
      return $t('asset.enum.coverageType.cyber');
    default:
      return '-';
  }
}

const statusOptions = [
  {
    value: 'INSURANCE_POLICY_STATUS_ACTIVE',
    label: $t('asset.enum.insurancePolicyStatus.active'),
  },
  {
    value: 'INSURANCE_POLICY_STATUS_EXPIRED',
    label: $t('asset.enum.insurancePolicyStatus.expired'),
  },
  {
    value: 'INSURANCE_POLICY_STATUS_CANCELLED',
    label: $t('asset.enum.insurancePolicyStatus.cancelled'),
  },
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
      label: $t('asset.page.insurance.status'),
      componentProps: {
        options: statusOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<InsurancePolicy> = {
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
        const resp = await insuranceStore.listPolicies(
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
      title: $t('asset.page.insurance.name'),
      field: 'name',
      minWidth: 150,
    },
    {
      title: $t('asset.page.insurance.policyNumber'),
      field: 'policyNumber',
      width: 140,
    },
    {
      title: $t('asset.page.insurance.provider'),
      field: 'provider',
      width: 140,
    },
    {
      title: $t('asset.page.insurance.coverageType'),
      field: 'coverageType',
      width: 150,
      slots: { default: 'coverageType' },
    },
    {
      title: $t('asset.page.insurance.status'),
      field: 'status',
      width: 120,
      slots: { default: 'status' },
    },
    {
      title: $t('asset.page.insurance.validFrom'),
      field: 'validFrom',
      width: 120,
    },
    {
      title: $t('asset.page.insurance.validTo'),
      field: 'validTo',
      width: 120,
    },
    {
      title: $t('asset.page.insurance.premiumAmount'),
      field: 'premiumAmount',
      width: 120,
    },
    {
      title: $t('asset.page.insurance.assetCount'),
      field: 'assetCount',
      width: 80,
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

const [InsuranceDrawerComponent, insuranceDrawerApi] = useVbenDrawer({
  connectedComponent: InsuranceDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function openDrawer(
  row: InsurancePolicy,
  mode: 'create' | 'edit' | 'view',
) {
  insuranceDrawerApi.setData({ row, mode });
  insuranceDrawerApi.open();
}

function handleView(row: InsurancePolicy) {
  openDrawer(row, 'view');
}

function handleEdit(row: InsurancePolicy) {
  openDrawer(row, 'edit');
}

function handleCreate() {
  openDrawer({} as InsurancePolicy, 'create');
}

async function handleDelete(row: InsurancePolicy) {
  if (!row.id) return;
  try {
    await insuranceStore.deletePolicy(row.id);
    notification.success({
      message: $t('asset.page.insurance.deleteSuccess'),
    });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('asset.page.insurance.title')">
      <template #toolbar-tools>
        <Button class="mr-2" type="primary" @click="handleCreate">
          {{ $t('asset.page.insurance.create') }}
        </Button>
      </template>
      <template #coverageType="{ row }">
        {{ getCoverageTypeLabel(row.coverageType) }}
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
            :title="$t('asset.page.insurance.confirmDelete')"
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

    <InsuranceDrawerComponent />
  </Page>
</template>
