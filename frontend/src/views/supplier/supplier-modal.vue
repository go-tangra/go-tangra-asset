<script lang="ts" setup>
import { ref, computed } from 'vue';

import { useVbenModal } from 'shell/vben/common-ui';

import {
  Form,
  FormItem,
  Input,
  Button,
  notification,
  Textarea,
  Select,
  Descriptions,
  DescriptionsItem,
  Tag,
  Tabs,
  TabPane,
} from 'ant-design-vue';

import type { Supplier } from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetSupplierStore } from '../../stores/asset-supplier.state';

const supplierStore = useAssetSupplierStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: Supplier;
}>();
const loading = ref(false);
const activeTab = ref('general');

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

const formState = ref({
  name: '',
  code: '',
  status: 1 as number,
  address: '',
  city: '',
  state: '',
  country: '',
  postalCode: '',
  contactPerson: '',
  telephone: '',
  email: '',
  website: '',
  notes: '',
  metadata: '',
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('asset.page.supplier.create');
    case 'edit':
      return $t('asset.page.supplier.edit');
    default:
      return $t('asset.page.supplier.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await supplierStore.createSupplier({
        name: formState.value.name,
        code: formState.value.code || undefined,
        status: formState.value.status,
        address: formState.value.address || undefined,
        city: formState.value.city || undefined,
        state: formState.value.state || undefined,
        country: formState.value.country || undefined,
        postalCode: formState.value.postalCode || undefined,
        contactPerson: formState.value.contactPerson || undefined,
        telephone: formState.value.telephone || undefined,
        email: formState.value.email || undefined,
        website: formState.value.website || undefined,
        notes: formState.value.notes || undefined,
        metadata: formState.value.metadata ? JSON.parse(formState.value.metadata) : undefined,
      });
      notification.success({
        message: $t('asset.page.supplier.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await supplierStore.updateSupplier(
        data.value.row.id,
        {
          name: formState.value.name,
          code: formState.value.code || undefined,
          status: formState.value.status,
          address: formState.value.address || undefined,
          city: formState.value.city || undefined,
          state: formState.value.state || undefined,
          country: formState.value.country || undefined,
          postalCode: formState.value.postalCode || undefined,
          contactPerson: formState.value.contactPerson || undefined,
          telephone: formState.value.telephone || undefined,
          email: formState.value.email || undefined,
          website: formState.value.website || undefined,
          notes: formState.value.notes || undefined,
          metadata: formState.value.metadata ? JSON.parse(formState.value.metadata) : undefined,
        },
        [
          'name',
          'code',
          'status',
          'address',
          'city',
          'state',
          'country',
          'postalCode',
          'contactPerson',
          'telephone',
          'email',
          'website',
          'notes',
          'metadata',
        ],
      );
      notification.success({
        message: $t('asset.page.supplier.updateSuccess'),
      });
    }
    modalApi.close();
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
    code: '',
    status: 1,
    address: '',
    city: '',
    state: '',
    country: '',
    postalCode: '',
    contactPerson: '',
    telephone: '',
    email: '',
    website: '',
    notes: '',
    metadata: '',
  };
}

const [Modal, modalApi] = useVbenModal({
  onCancel() {
    modalApi.close();
  },

  async onOpenChange(isOpen) {
    if (isOpen) {
      data.value = modalApi.getData() as {
        mode: 'create' | 'edit' | 'view';
        row?: Supplier;
      };

      activeTab.value = 'general';

      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name ?? '',
          code: data.value.row.code ?? '',
          status: Number(data.value.row.status) || 1,
          address: data.value.row.address ?? '',
          city: data.value.row.city ?? '',
          state: data.value.row.state ?? '',
          country: data.value.row.country ?? '',
          postalCode: data.value.row.postalCode ?? '',
          contactPerson: data.value.row.contactPerson ?? '',
          telephone: data.value.row.telephone ?? '',
          email: data.value.row.email ?? '',
          website: data.value.row.website ?? '',
          notes: data.value.row.notes ?? '',
          metadata: data.value.row.metadata ? JSON.stringify(data.value.row.metadata, null, 2) : '',
        };
      }
    }
  },
});

const supplier = computed(() => data.value?.row);
</script>

<template>
  <Modal :title="title" :footer="false" class="w-[700px]">
    <!-- View Mode -->
    <template v-if="supplier && isViewMode">
      <Tabs v-model:activeKey="activeTab">
        <TabPane key="general" :tab="$t('asset.page.supplier.tabGeneral')">
          <Descriptions :column="1" bordered size="small">
            <DescriptionsItem :label="$t('asset.page.supplier.name')">
              {{ supplier.name }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.code')">
              {{ supplier.code || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.status')">
              <Tag :color="statusToColor(supplier.status)">
                {{ statusToName(supplier.status) }}
              </Tag>
            </DescriptionsItem>
            <DescriptionsItem
              v-if="supplier.notes"
              :label="$t('asset.page.supplier.notes')"
            >
              {{ supplier.notes }}
            </DescriptionsItem>
            <DescriptionsItem
              v-if="supplier.metadata && Object.keys(supplier.metadata).length > 0"
              :label="$t('asset.page.supplier.metadata')"
            >
              <pre class="m-0 whitespace-pre-wrap text-xs">{{ JSON.stringify(supplier.metadata, null, 2) }}</pre>
            </DescriptionsItem>
          </Descriptions>
        </TabPane>

        <TabPane key="address" :tab="$t('asset.page.supplier.tabAddress')">
          <Descriptions :column="1" bordered size="small">
            <DescriptionsItem :label="$t('asset.page.supplier.address')">
              {{ supplier.address || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.city')">
              {{ supplier.city || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.state')">
              {{ supplier.state || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.country')">
              {{ supplier.country || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.postalCode')">
              {{ supplier.postalCode || '-' }}
            </DescriptionsItem>
          </Descriptions>
        </TabPane>

        <TabPane key="contact" :tab="$t('asset.page.supplier.tabContact')">
          <Descriptions :column="1" bordered size="small">
            <DescriptionsItem
              :label="$t('asset.page.supplier.contactPerson')"
            >
              {{ supplier.contactPerson || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.telephone')">
              {{ supplier.telephone || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.email')">
              {{ supplier.email || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.supplier.website')">
              {{ supplier.website || '-' }}
            </DescriptionsItem>
          </Descriptions>
        </TabPane>
      </Tabs>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <Tabs v-model:activeKey="activeTab">
          <TabPane key="general" :tab="$t('asset.page.supplier.tabGeneral')">
            <FormItem
              :label="$t('asset.page.supplier.name')"
              name="name"
              :rules="[
                { required: true, message: $t('ui.formRules.required') },
              ]"
            >
              <Input
                v-model:value="formState.name"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.supplier.code')" name="code">
              <Input
                v-model:value="formState.code"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="100"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.supplier.status')"
              name="status"
            >
              <Select
                v-model:value="formState.status"
                :options="statusOptions"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.supplier.notes')" name="notes">
              <Textarea
                v-model:value="formState.notes"
                :rows="3"
                :maxlength="1024"
                :placeholder="$t('ui.placeholder.input')"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.supplier.metadata')" name="metadata">
              <Textarea
                v-model:value="formState.metadata"
                :rows="4"
                placeholder='{"key": "value"}'
                style="font-family: monospace; font-size: 12px"
              />
            </FormItem>
          </TabPane>

          <TabPane key="address" :tab="$t('asset.page.supplier.tabAddress')">
            <FormItem
              :label="$t('asset.page.supplier.address')"
              name="address"
            >
              <Input
                v-model:value="formState.address"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="500"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.supplier.city')" name="city">
              <Input
                v-model:value="formState.city"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="100"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.supplier.state')" name="state">
              <Input
                v-model:value="formState.state"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="100"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.supplier.country')"
              name="country"
            >
              <Input
                v-model:value="formState.country"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="100"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.supplier.postalCode')"
              name="postalCode"
            >
              <Input
                v-model:value="formState.postalCode"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="20"
              />
            </FormItem>
          </TabPane>

          <TabPane key="contact" :tab="$t('asset.page.supplier.tabContact')">
            <FormItem
              :label="$t('asset.page.supplier.contactPerson')"
              name="contactPerson"
            >
              <Input
                v-model:value="formState.contactPerson"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.supplier.telephone')"
              name="telephone"
            >
              <Input
                v-model:value="formState.telephone"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="50"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.supplier.email')" name="email">
              <Input
                v-model:value="formState.email"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.supplier.website')"
              name="website"
            >
              <Input
                v-model:value="formState.website"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="500"
              />
            </FormItem>
          </TabPane>
        </Tabs>

        <FormItem class="mt-4">
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
  </Modal>
</template>
