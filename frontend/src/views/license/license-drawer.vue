<script lang="ts" setup>
import { ref, computed } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';

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
} from 'ant-design-vue';

import type { License, LicenseStatus } from '../../api/services';
import { SupplierService } from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetLicenseStore } from '../../stores/asset-license.state';

const licenseStore = useAssetLicenseStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: License;
}>();
const loading = ref(false);

interface SelectOption {
  value: string;
  label: string;
}

const supplierOptions = ref<SelectOption[]>([]);

const statusOptions = [
  { value: 'LICENSE_STATUS_ACTIVE', label: $t('asset.enum.licenseStatus.active') },
  { value: 'LICENSE_STATUS_EXPIRED', label: $t('asset.enum.licenseStatus.expired') },
  { value: 'LICENSE_STATUS_SUSPENDED', label: $t('asset.enum.licenseStatus.suspended') },
];

async function loadOptions() {
  try {
    const supResp = await SupplierService.list({ noPaging: true });
    supplierOptions.value = (supResp.items ?? []).map((s) => ({
      value: s.id,
      label: s.name,
    }));
  } catch (e) {
    console.error('Failed to load options:', e);
  }
}

function getOptionLabel(options: SelectOption[], id: string | undefined) {
  if (!id) return '-';
  return options.find((o) => o.value === id)?.label ?? id;
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

const formState = ref({
  name: '',
  supplierId: undefined as string | undefined,
  purchaseDate: '',
  purchaseCost: undefined as number | undefined,
  orderNumber: '',
  validFrom: '',
  validTo: '',
  notes: '',
  metadata: '',
  status: 'LICENSE_STATUS_ACTIVE' as string,
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('asset.page.license.create');
    case 'edit':
      return $t('asset.page.license.edit');
    default:
      return $t('asset.page.license.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await licenseStore.createLicense({
        name: formState.value.name,
        supplierId: formState.value.supplierId,
        purchaseDate: formState.value.purchaseDate ? `${formState.value.purchaseDate}T00:00:00Z` : undefined,
        purchaseCost: formState.value.purchaseCost,
        orderNumber: formState.value.orderNumber || undefined,
        validFrom: formState.value.validFrom ? `${formState.value.validFrom}T00:00:00Z` : undefined,
        validTo: formState.value.validTo ? `${formState.value.validTo}T00:00:00Z` : undefined,
        notes: formState.value.notes || undefined,
        metadata: formState.value.metadata ? JSON.parse(formState.value.metadata) : undefined,
        status: (formState.value.status as LicenseStatus) || undefined,
      });
      notification.success({
        message: $t('asset.page.license.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await licenseStore.updateLicense(
        data.value.row.id,
        {
          name: formState.value.name,
          supplierId: formState.value.supplierId,
          purchaseDate: formState.value.purchaseDate ? `${formState.value.purchaseDate}T00:00:00Z` : undefined,
          purchaseCost: formState.value.purchaseCost,
          orderNumber: formState.value.orderNumber || undefined,
          validFrom: formState.value.validFrom ? `${formState.value.validFrom}T00:00:00Z` : undefined,
          validTo: formState.value.validTo ? `${formState.value.validTo}T00:00:00Z` : undefined,
          notes: formState.value.notes || undefined,
          metadata: formState.value.metadata ? JSON.parse(formState.value.metadata) : undefined,
          status: (formState.value.status as LicenseStatus) || undefined,
        },
        [
          'name',
          'supplierId',
          'purchaseDate',
          'purchaseCost',
          'orderNumber',
          'validFrom',
          'validTo',
          'notes',
          'metadata',
          'status',
        ],
      );
      notification.success({
        message: $t('asset.page.license.updateSuccess'),
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
    supplierId: undefined,
    purchaseDate: '',
    purchaseCost: undefined,
    orderNumber: '',
    validFrom: '',
    validTo: '',
    notes: '',
    metadata: '',
    status: 'LICENSE_STATUS_ACTIVE',
  };
}

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },

  async onOpenChange(isOpen) {
    if (isOpen) {
      data.value = drawerApi.getData() as {
        mode: 'create' | 'edit' | 'view';
        row?: License;
      };

      await loadOptions();

      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name ?? '',
          supplierId: data.value.row.supplierId || undefined,
          purchaseDate: data.value.row.purchaseDate ?? '',
          purchaseCost: data.value.row.purchaseCost,
          orderNumber: data.value.row.orderNumber ?? '',
          validFrom: data.value.row.validFrom ?? '',
          validTo: data.value.row.validTo ?? '',
          notes: data.value.row.notes ?? '',
          metadata: data.value.row.metadata ? JSON.stringify(data.value.row.metadata, null, 2) : '',
          status: data.value.row.status ?? 'LICENSE_STATUS_ACTIVE',
        };
      }
    }
  },
});

const license = computed(() => data.value?.row);
</script>

<template>
  <Drawer :title="title" :footer="false">
    <!-- View Mode -->
    <template v-if="license && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('asset.page.license.name')">
          {{ license.name }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.license.supplierId')">
          {{ getOptionLabel(supplierOptions, license.supplierId) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.license.status')">
          <Tag :color="getStatusColor(license.status)">
            {{ getStatusLabel(license.status) }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.license.purchaseDate')">
          {{ license.purchaseDate || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.license.purchaseCost')">
          {{ license.purchaseCost ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.license.orderNumber')">
          {{ license.orderNumber || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.license.validFrom')">
          {{ license.validFrom || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.license.validTo')">
          {{ license.validTo || '-' }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="license.notes"
          :label="$t('asset.page.license.notes')"
        >
          {{ license.notes }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="license.metadata && Object.keys(license.metadata).length > 0"
          :label="$t('asset.page.license.metadata')"
        >
          <pre class="m-0 whitespace-pre-wrap text-xs">{{ JSON.stringify(license.metadata, null, 2) }}</pre>
        </DescriptionsItem>
      </Descriptions>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('asset.page.license.name')"
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
          :label="$t('asset.page.license.supplierId')"
          name="supplierId"
        >
          <Select
            v-model:value="formState.supplierId"
            :options="supplierOptions"
            :placeholder="$t('ui.placeholder.select')"
            allow-clear
            show-search
            :filter-option="
              (input: string, option: any) =>
                option.label.toLowerCase().includes(input.toLowerCase())
            "
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.license.status')"
          name="status"
        >
          <Select
            v-model:value="formState.status"
            :options="statusOptions"
            :placeholder="$t('ui.placeholder.select')"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.license.purchaseDate')"
          name="purchaseDate"
        >
          <DatePicker
            v-model:value="formState.purchaseDate"
            value-format="YYYY-MM-DD"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.license.purchaseCost')"
          name="purchaseCost"
        >
          <InputNumber
            v-model:value="formState.purchaseCost"
            :min="0"
            :precision="2"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.license.orderNumber')"
          name="orderNumber"
        >
          <Input
            v-model:value="formState.orderNumber"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.license.validFrom')"
          name="validFrom"
        >
          <DatePicker
            v-model:value="formState.validFrom"
            value-format="YYYY-MM-DD"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.license.validTo')"
          name="validTo"
        >
          <DatePicker
            v-model:value="formState.validTo"
            value-format="YYYY-MM-DD"
            style="width: 100%"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.license.notes')" name="notes">
          <Textarea
            v-model:value="formState.notes"
            :rows="3"
            :maxlength="1024"
            :placeholder="$t('ui.placeholder.input')"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.license.metadata')" name="metadata">
          <Textarea
            v-model:value="formState.metadata"
            :rows="4"
            :placeholder='`{"key": "value"}`'
            style="font-family: monospace"
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
</template>
