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
  DatePicker,
} from 'ant-design-vue';

import type { Consumable } from '../../api/services';
import {
  CategoryService,
  SupplierService,
  LocationService,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetConsumableStore } from '../../stores/asset-consumable.state';

const consumableStore = useAssetConsumableStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: Consumable;
}>();
const loading = ref(false);

interface SelectOption {
  value: string;
  label: string;
}

const categoryOptions = ref<SelectOption[]>([]);
const supplierOptions = ref<SelectOption[]>([]);
const locationOptions = ref<SelectOption[]>([]);

async function loadOptions() {
  try {
    const [catResp, supResp, locResp] = await Promise.all([
      CategoryService.list({ noPaging: true }),
      SupplierService.list({ noPaging: true }),
      LocationService.list({ noPaging: true }),
    ]);
    categoryOptions.value = ((catResp as any).categories ?? (catResp as any).items ?? []).map((c: any) => ({
      value: c.id,
      label: c.name,
    }));
    supplierOptions.value = ((supResp as any).suppliers ?? (supResp as any).items ?? []).map((s: any) => ({
      value: s.id,
      label: s.name,
    }));
    locationOptions.value = ((locResp as any).locations ?? (locResp as any).items ?? []).map((l: any) => ({
      value: l.id,
      label: l.name,
    }));
  } catch (e) {
    console.error('Failed to load options:', e);
  }
}

function getOptionLabel(options: SelectOption[], id: string | undefined) {
  if (!id) return '-';
  return options.find((o) => o.value === id)?.label ?? id;
}

const formState = ref({
  name: '',
  description: '',
  categoryId: undefined as string | undefined,
  supplierId: undefined as string | undefined,
  locationId: undefined as string | undefined,
  modelName: '',
  modelNumber: '',
  amount: 0,
  minAmount: 0,
  purchaseDate: '',
  purchaseCost: undefined as number | undefined,
  orderNumber: '',
  notes: '',
  metadata: '',
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('asset.page.consumable.create');
    case 'edit':
      return $t('asset.page.consumable.edit');
    default:
      return $t('asset.page.consumable.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await consumableStore.createConsumable({
        name: formState.value.name,
        description: formState.value.description || undefined,
        categoryId: formState.value.categoryId,
        supplierId: formState.value.supplierId,
        locationId: formState.value.locationId,
        modelName: formState.value.modelName || undefined,
        modelNumber: formState.value.modelNumber || undefined,
        amount: formState.value.amount,
        minAmount: formState.value.minAmount,
        purchaseDate: formState.value.purchaseDate ? `${formState.value.purchaseDate}T00:00:00Z` : undefined,
        purchaseCost: formState.value.purchaseCost,
        orderNumber: formState.value.orderNumber || undefined,
        notes: formState.value.notes || undefined,
        metadata: formState.value.metadata ? JSON.parse(formState.value.metadata) : undefined,
      });
      notification.success({
        message: $t('asset.page.consumable.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await consumableStore.updateConsumable(
        data.value.row.id,
        {
          name: formState.value.name,
          description: formState.value.description || undefined,
          categoryId: formState.value.categoryId,
          supplierId: formState.value.supplierId,
          locationId: formState.value.locationId,
          modelName: formState.value.modelName || undefined,
          modelNumber: formState.value.modelNumber || undefined,
          amount: formState.value.amount,
          minAmount: formState.value.minAmount,
          purchaseDate: formState.value.purchaseDate ? `${formState.value.purchaseDate}T00:00:00Z` : undefined,
          purchaseCost: formState.value.purchaseCost,
          orderNumber: formState.value.orderNumber || undefined,
          notes: formState.value.notes || undefined,
          metadata: formState.value.metadata ? JSON.parse(formState.value.metadata) : undefined,
        },
        [
          'name',
          'description',
          'categoryId',
          'supplierId',
          'locationId',
          'modelName',
          'modelNumber',
          'amount',
          'minAmount',
          'purchaseDate',
          'purchaseCost',
          'orderNumber',
          'notes',
          'metadata',
        ],
      );
      notification.success({
        message: $t('asset.page.consumable.updateSuccess'),
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
    description: '',
    categoryId: undefined,
    supplierId: undefined,
    locationId: undefined,
    modelName: '',
    modelNumber: '',
    amount: 0,
    minAmount: 0,
    purchaseDate: '',
    purchaseCost: undefined,
    orderNumber: '',
    notes: '',
    metadata: '',
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
        row?: Consumable;
      };

      await loadOptions();

      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name ?? '',
          description: data.value.row.description ?? '',
          categoryId: data.value.row.categoryId || undefined,
          supplierId: data.value.row.supplierId || undefined,
          locationId: data.value.row.locationId || undefined,
          modelName: data.value.row.modelName ?? '',
          modelNumber: data.value.row.modelNumber ?? '',
          amount: data.value.row.amount ?? 0,
          minAmount: data.value.row.minAmount ?? 0,
          purchaseDate: data.value.row.purchaseDate ?? '',
          purchaseCost: data.value.row.purchaseCost,
          orderNumber: data.value.row.orderNumber ?? '',
          notes: data.value.row.notes ?? '',
          metadata: data.value.row.metadata ? JSON.stringify(data.value.row.metadata, null, 2) : '',
        };
      }
    }
  },
});

const consumable = computed(() => data.value?.row);
</script>

<template>
  <Drawer :title="title" :footer="false">
    <!-- View Mode -->
    <template v-if="consumable && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('asset.page.consumable.name')">
          {{ consumable.name }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.description')">
          {{ consumable.description || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.categoryId')">
          {{ getOptionLabel(categoryOptions, consumable.categoryId) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.supplierId')">
          {{ getOptionLabel(supplierOptions, consumable.supplierId) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.locationId')">
          {{ getOptionLabel(locationOptions, consumable.locationId) }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.modelName')">
          {{ consumable.modelName || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.modelNumber')">
          {{ consumable.modelNumber || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.amount')">
          <span
            :style="{
              color:
                consumable.minAmount &&
                (consumable.amount ?? 0) < consumable.minAmount
                  ? '#FF4D4F'
                  : undefined,
              fontWeight:
                consumable.minAmount &&
                (consumable.amount ?? 0) < consumable.minAmount
                  ? 'bold'
                  : undefined,
            }"
          >
            {{ consumable.amount ?? 0 }}
          </span>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.minAmount')">
          {{ consumable.minAmount ?? 0 }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.purchaseDate')">
          {{ consumable.purchaseDate || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.purchaseCost')">
          {{ consumable.purchaseCost ?? '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.consumable.orderNumber')">
          {{ consumable.orderNumber || '-' }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="consumable.notes"
          :label="$t('asset.page.consumable.notes')"
        >
          {{ consumable.notes }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="consumable.metadata && Object.keys(consumable.metadata).length > 0"
          :label="$t('asset.page.consumable.metadata')"
        >
          <pre class="m-0 whitespace-pre-wrap text-xs">{{ JSON.stringify(consumable.metadata, null, 2) }}</pre>
        </DescriptionsItem>
      </Descriptions>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('asset.page.consumable.name')"
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
          :label="$t('asset.page.consumable.description')"
          name="description"
        >
          <Textarea
            v-model:value="formState.description"
            :rows="3"
            :maxlength="1024"
            :placeholder="$t('ui.placeholder.input')"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.consumable.categoryId')"
          name="categoryId"
        >
          <Select
            v-model:value="formState.categoryId"
            :options="categoryOptions"
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
          :label="$t('asset.page.consumable.supplierId')"
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
          :label="$t('asset.page.consumable.locationId')"
          name="locationId"
        >
          <Select
            v-model:value="formState.locationId"
            :options="locationOptions"
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
          :label="$t('asset.page.consumable.modelName')"
          name="modelName"
        >
          <Input
            v-model:value="formState.modelName"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.consumable.modelNumber')"
          name="modelNumber"
        >
          <Input
            v-model:value="formState.modelNumber"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.consumable.amount')" name="amount">
          <InputNumber
            v-model:value="formState.amount"
            :min="0"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.consumable.minAmount')"
          name="minAmount"
        >
          <InputNumber
            v-model:value="formState.minAmount"
            :min="0"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.consumable.purchaseDate')"
          name="purchaseDate"
        >
          <DatePicker
            v-model:value="formState.purchaseDate"
            value-format="YYYY-MM-DD"
            style="width: 100%"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.consumable.purchaseCost')"
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
          :label="$t('asset.page.consumable.orderNumber')"
          name="orderNumber"
        >
          <Input
            v-model:value="formState.orderNumber"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.consumable.notes')" name="notes">
          <Textarea
            v-model:value="formState.notes"
            :rows="3"
            :maxlength="1024"
            :placeholder="$t('ui.placeholder.input')"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.consumable.metadata')"
          name="metadata"
        >
          <Textarea
            v-model:value="formState.metadata"
            :rows="4"
            style="font-family: monospace"
            placeholder='{"key": "value"}'
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
