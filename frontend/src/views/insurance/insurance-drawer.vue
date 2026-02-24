<script lang="ts" setup>
import { h, ref, computed } from 'vue';

import { useVbenDrawer, useVbenModal } from 'shell/vben/common-ui';
import { LucideTrash } from 'shell/vben/icons';

import {
  Form,
  FormItem,
  Input,
  InputNumber,
  Button,
  notification,
  Textarea,
  Select,
  Descriptions,
  DescriptionsItem,
  Tag,
  DatePicker,
  Table,
  Space,
} from 'ant-design-vue';

import type {
  InsurancePolicy,
  InsurancePolicyStatus,
  CoverageType,
  PolicyAsset,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetInsuranceStore } from '../../stores/asset-insurance.state';

import AssetSelectModal from './asset-select-modal.vue';

const insuranceStore = useAssetInsuranceStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: InsurancePolicy;
}>();
const loading = ref(false);
const policyAssets = ref<PolicyAsset[]>([]);

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

const coverageTypeOptions = [
  {
    value: 'COVERAGE_TYPE_ALL_RISK',
    label: $t('asset.enum.coverageType.allRisk'),
  },
  {
    value: 'COVERAGE_TYPE_FIRE_THEFT',
    label: $t('asset.enum.coverageType.fireTheft'),
  },
  {
    value: 'COVERAGE_TYPE_LIABILITY',
    label: $t('asset.enum.coverageType.liability'),
  },
  {
    value: 'COVERAGE_TYPE_EQUIPMENT_BREAKDOWN',
    label: $t('asset.enum.coverageType.equipmentBreakdown'),
  },
  {
    value: 'COVERAGE_TYPE_CYBER',
    label: $t('asset.enum.coverageType.cyber'),
  },
];

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

const formState = ref({
  name: '',
  policyNumber: '',
  provider: '',
  coverageType: undefined as string | undefined,
  premiumAmount: undefined as number | undefined,
  deductible: undefined as number | undefined,
  coverageLimit: undefined as number | undefined,
  validFrom: '',
  validTo: '',
  notes: '',
  status: 'INSURANCE_POLICY_STATUS_ACTIVE' as string,
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('asset.page.insurance.create');
    case 'edit':
      return $t('asset.page.insurance.edit');
    default:
      return $t('asset.page.insurance.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

const assetColumns = [
  {
    title: $t('asset.page.insurance.assetTag'),
    dataIndex: 'assetTag',
    key: 'assetTag',
  },
  {
    title: $t('asset.page.insurance.assetName'),
    dataIndex: 'assetName',
    key: 'assetName',
  },
  {
    title: $t('asset.page.insurance.modelName'),
    dataIndex: 'modelName',
    key: 'modelName',
  },
  {
    title: $t('asset.page.insurance.coveredValue'),
    dataIndex: 'coveredValue',
    key: 'coveredValue',
  },
  {
    title: $t('ui.table.action'),
    key: 'action',
    width: 80,
  },
];

async function loadPolicyAssets() {
  if (!data.value?.row?.id) return;
  try {
    const resp = await insuranceStore.listPolicyAssets(data.value.row.id);
    policyAssets.value = resp.items ?? [];
  } catch (e) {
    console.error('Failed to load policy assets:', e);
  }
}

async function handleRemoveAsset(record: PolicyAsset) {
  if (!data.value?.row?.id) return;
  try {
    await insuranceStore.removeAssetFromPolicy(
      data.value.row.id,
      record.assetId,
    );
    notification.success({
      message: $t('asset.page.insurance.removeAssetSuccess'),
    });
    await loadPolicyAssets();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await insuranceStore.createPolicy({
        name: formState.value.name,
        policyNumber: formState.value.policyNumber || undefined,
        provider: formState.value.provider || undefined,
        coverageType:
          (formState.value.coverageType as CoverageType) || undefined,
        premiumAmount: formState.value.premiumAmount,
        deductible: formState.value.deductible,
        coverageLimit: formState.value.coverageLimit,
        validFrom: formState.value.validFrom
          ? `${formState.value.validFrom}T00:00:00Z`
          : undefined,
        validTo: formState.value.validTo
          ? `${formState.value.validTo}T00:00:00Z`
          : undefined,
        notes: formState.value.notes || undefined,
        status:
          (formState.value.status as InsurancePolicyStatus) || undefined,
      });
      notification.success({
        message: $t('asset.page.insurance.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await insuranceStore.updatePolicy(
        data.value.row.id,
        {
          name: formState.value.name,
          policyNumber: formState.value.policyNumber || undefined,
          provider: formState.value.provider || undefined,
          coverageType:
            (formState.value.coverageType as CoverageType) || undefined,
          premiumAmount: formState.value.premiumAmount,
          deductible: formState.value.deductible,
          coverageLimit: formState.value.coverageLimit,
          validFrom: formState.value.validFrom
            ? `${formState.value.validFrom}T00:00:00Z`
            : undefined,
          validTo: formState.value.validTo
            ? `${formState.value.validTo}T00:00:00Z`
            : undefined,
          notes: formState.value.notes || undefined,
          status:
            (formState.value.status as InsurancePolicyStatus) || undefined,
        },
        [
          'name',
          'policyNumber',
          'provider',
          'coverageType',
          'premiumAmount',
          'deductible',
          'coverageLimit',
          'validFrom',
          'validTo',
          'notes',
          'status',
        ],
      );
      notification.success({
        message: $t('asset.page.insurance.updateSuccess'),
      });
    }
    drawerApi.close();
  } catch {
    notification.error({
      message: isCreateMode.value
        ? $t('ui.notification.create_failed')
        : $t('ui.notification.update_failed'),
    });
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  formState.value = {
    name: '',
    policyNumber: '',
    provider: '',
    coverageType: undefined,
    premiumAmount: undefined,
    deductible: undefined,
    coverageLimit: undefined,
    validFrom: '',
    validTo: '',
    notes: '',
    status: 'INSURANCE_POLICY_STATUS_ACTIVE',
  };
}

const [AssetSelectModalComponent, assetSelectModalApi] = useVbenModal({
  connectedComponent: AssetSelectModal,
  async onOpenChange(isOpen) {
    if (!isOpen) {
      await loadPolicyAssets();
    }
  },
});

function handleAddAsset() {
  if (!data.value?.row?.id) return;
  assetSelectModalApi.setData({ policyId: data.value.row.id });
  assetSelectModalApi.open();
}

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },

  async onOpenChange(isOpen) {
    if (isOpen) {
      data.value = drawerApi.getData() as {
        mode: 'create' | 'edit' | 'view';
        row?: InsurancePolicy;
      };

      if (data.value?.mode === 'create') {
        resetForm();
        policyAssets.value = [];
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name ?? '',
          policyNumber: data.value.row.policyNumber ?? '',
          provider: data.value.row.provider ?? '',
          coverageType: data.value.row.coverageType || undefined,
          premiumAmount: data.value.row.premiumAmount,
          deductible: data.value.row.deductible,
          coverageLimit: data.value.row.coverageLimit,
          validFrom: data.value.row.validFrom ?? '',
          validTo: data.value.row.validTo ?? '',
          notes: data.value.row.notes ?? '',
          status:
            data.value.row.status ?? 'INSURANCE_POLICY_STATUS_ACTIVE',
        };
        await loadPolicyAssets();
      }
    }
  },
});

const policy = computed(() => data.value?.row);
</script>

<template>
  <Drawer :title="title" :footer="false">
    <!-- View Mode -->
    <template v-if="policy && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('asset.page.insurance.name')">
          {{ policy.name }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.policyNumber')">
          {{ policy.policyNumber || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.provider')">
          {{ policy.provider || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.coverageType')">
          {{ getCoverageTypeLabel(policy.coverageType) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.status')">
          <Tag :color="getStatusColor(policy.status)">
            {{ getStatusLabel(policy.status) }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.premiumAmount')">
          {{ policy.premiumAmount ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.deductible')">
          {{ policy.deductible ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.coverageLimit')">
          {{ policy.coverageLimit ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.validFrom')">
          {{ policy.validFrom || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.insurance.validTo')">
          {{ policy.validTo || '-' }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="policy.notes"
          :label="$t('asset.page.insurance.notes')"
        >
          {{ policy.notes }}
        </DescriptionsItem>
      </Descriptions>

      <!-- Covered Assets Section -->
      <div class="mt-6">
        <div class="mb-3 flex items-center justify-between">
          <h3 class="text-base font-medium">
            {{ $t('asset.page.insurance.coveredAssets') }}
          </h3>
          <Button type="primary" size="small" @click="handleAddAsset">
            {{ $t('asset.page.insurance.addAsset') }}
          </Button>
        </div>
        <Table
          :columns="assetColumns"
          :data-source="policyAssets"
          :pagination="false"
          size="small"
          row-key="assetId"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'coveredValue'">
              {{ record.coveredValue ?? '-' }}
            </template>
            <template v-if="column.key === 'action'">
              <a-popconfirm
                :cancel-text="$t('ui.button.cancel')"
                :ok-text="$t('ui.button.ok')"
                :title="$t('asset.page.insurance.confirmRemoveAsset')"
                @confirm="handleRemoveAsset(record)"
              >
                <Button
                  danger
                  type="link"
                  size="small"
                  :icon="h(LucideTrash)"
                />
              </a-popconfirm>
            </template>
          </template>
        </Table>
      </div>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('asset.page.insurance.name')"
          name="name"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Input
            v-model:value="formState.name"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.policyNumber')"
          name="policyNumber"
        >
          <Input
            v-model:value="formState.policyNumber"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.provider')"
          name="provider"
        >
          <Input
            v-model:value="formState.provider"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.coverageType')"
          name="coverageType"
        >
          <Select
            v-model:value="formState.coverageType"
            :options="coverageTypeOptions"
            :placeholder="$t('ui.placeholder.select')"
            allow-clear
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.status')"
          name="status"
        >
          <Select
            v-model:value="formState.status"
            :options="statusOptions"
            :placeholder="$t('ui.placeholder.select')"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.premiumAmount')"
          name="premiumAmount"
        >
          <InputNumber
            v-model:value="formState.premiumAmount"
            :min="0"
            :precision="2"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.deductible')"
          name="deductible"
        >
          <InputNumber
            v-model:value="formState.deductible"
            :min="0"
            :precision="2"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.coverageLimit')"
          name="coverageLimit"
        >
          <InputNumber
            v-model:value="formState.coverageLimit"
            :min="0"
            :precision="2"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.validFrom')"
          name="validFrom"
        >
          <DatePicker
            v-model:value="formState.validFrom"
            value-format="YYYY-MM-DD"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.insurance.validTo')"
          name="validTo"
        >
          <DatePicker
            v-model:value="formState.validTo"
            value-format="YYYY-MM-DD"
            style="width: 100%"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.insurance.notes')" name="notes">
          <Textarea
            v-model:value="formState.notes"
            :rows="3"
            :maxlength="1024"
            :placeholder="$t('ui.placeholder.input')"
          />
        </FormItem>

        <FormItem>
          <Button type="primary" html-type="submit" :loading="loading" block>
            {{
              isCreateMode
                ? $t('ui.button.create', { moduleName: '' })
                : $t('ui.button.save')
            }}
          </Button>
        </FormItem>
      </Form>
    </template>
  </Drawer>

  <AssetSelectModalComponent />
</template>
