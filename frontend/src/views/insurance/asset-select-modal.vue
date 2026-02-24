<script lang="ts" setup>
import { ref } from 'vue';

import { useVbenModal } from 'shell/vben/common-ui';

import {
  Form,
  FormItem,
  Select,
  InputNumber,
  Input,
  Button,
  notification,
} from 'ant-design-vue';

import type { Asset } from '../../api/services';
import { AssetService } from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetInsuranceStore } from '../../stores/asset-insurance.state';

const insuranceStore = useAssetInsuranceStore();

const policyId = ref('');
const loading = ref(false);

interface AssetOption {
  value: string;
  label: string;
  asset: Asset;
}

const assetOptions = ref<AssetOption[]>([]);

const formState = ref({
  assetId: undefined as string | undefined,
  coveredValue: undefined as number | undefined,
  notes: '',
});

async function loadAssets() {
  try {
    const resp = await AssetService.list({ noPaging: true });
    assetOptions.value = (resp.items ?? []).map((a) => ({
      value: a.id,
      label: `${a.assetTag ?? ''} - ${a.name ?? ''}`.trim(),
      asset: a,
    }));
  } catch (e) {
    console.error('Failed to load assets:', e);
  }
}

function resetForm() {
  formState.value = {
    assetId: undefined,
    coveredValue: undefined,
    notes: '',
  };
}

async function handleSubmit() {
  if (!formState.value.assetId) return;
  loading.value = true;
  try {
    await insuranceStore.addAssetToPolicy(
      policyId.value,
      formState.value.assetId,
      formState.value.coveredValue,
      formState.value.notes || undefined,
    );
    notification.success({
      message: $t('asset.page.insurance.addAssetSuccess'),
    });
    modalApi.close();
  } catch {
    notification.error({ message: $t('ui.notification.create_failed') });
  } finally {
    loading.value = false;
  }
}

const [Modal, modalApi] = useVbenModal({
  onCancel() {
    modalApi.close();
  },

  async onOpenChange(isOpen) {
    if (isOpen) {
      const d = modalApi.getData() as { policyId: string };
      policyId.value = d.policyId;
      resetForm();
      await loadAssets();
    }
  },
});
</script>

<template>
  <Modal :title="$t('asset.page.insurance.addAsset')" :footer="false">
    <Form layout="vertical" :model="formState" @finish="handleSubmit">
      <FormItem
        :label="$t('asset.page.insurance.selectAsset')"
        name="assetId"
        :rules="[{ required: true, message: $t('ui.formRules.required') }]"
      >
        <Select
          v-model:value="formState.assetId"
          :options="assetOptions"
          :placeholder="$t('ui.placeholder.select')"
          show-search
          allow-clear
          :filter-option="
            (input: string, option: any) =>
              option.label.toLowerCase().includes(input.toLowerCase())
          "
        />
      </FormItem>

      <FormItem
        :label="$t('asset.page.insurance.coveredValue')"
        name="coveredValue"
      >
        <InputNumber
          v-model:value="formState.coveredValue"
          :min="0"
          :precision="2"
          style="width: 100%"
        />
      </FormItem>

      <FormItem
        :label="$t('asset.page.insurance.notes')"
        name="notes"
      >
        <Input
          v-model:value="formState.notes"
          :placeholder="$t('ui.placeholder.input')"
          :maxlength="512"
        />
      </FormItem>

      <FormItem>
        <Button type="primary" html-type="submit" :loading="loading" block>
          {{ $t('asset.page.insurance.addAsset') }}
        </Button>
      </FormItem>
    </Form>
  </Modal>
</template>
