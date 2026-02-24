<script lang="ts" setup>
import { ref, computed } from 'vue';

import { useVbenDrawer } from 'shell/vben/common-ui';

import {
  Form,
  FormItem,
  Input,
  Button,
  notification,
  Textarea,
  Select,
  TreeSelect,
  Descriptions,
  DescriptionsItem,
  Tag,
  Divider,
} from 'ant-design-vue';

import type {
  Location,
  LocationTreeNode,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetLocationStore } from '../../stores/asset-location.state';

const locationStore = useAssetLocationStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: Location;
  parentId?: string;
}>();
const loading = ref(false);

const statusOptions = computed(() => [
  {
    value: 'LOCATION_STATUS_ACTIVE',
    label: $t('asset.enum.locationStatus.active'),
  },
  {
    value: 'LOCATION_STATUS_PLANNED',
    label: $t('asset.enum.locationStatus.planned'),
  },
  {
    value: 'LOCATION_STATUS_DECOMMISSIONED',
    label: $t('asset.enum.locationStatus.decommissioned'),
  },
]);

function statusToColor(status: string | undefined) {
  switch (status) {
    case 'LOCATION_STATUS_ACTIVE':
      return '#52C41A';
    case 'LOCATION_STATUS_PLANNED':
      return '#FAAD14';
    case 'LOCATION_STATUS_DECOMMISSIONED':
      return '#8C8C8C';
    default:
      return '#C9CDD4';
  }
}

function statusToName(status: string | undefined) {
  const option = statusOptions.value.find((o) => o.value === status);
  return option?.label ?? status ?? '';
}

interface TreeSelectNode {
  value: string;
  title: string;
  children?: TreeSelectNode[];
}

const treeSelectData = ref<TreeSelectNode[]>([]);

function buildTreeSelectData(nodes: LocationTreeNode[]): TreeSelectNode[] {
  return nodes.map((node) => ({
    value: node.location?.id ?? '',
    title: node.location?.name ?? '',
    children: node.children ? buildTreeSelectData(node.children) : undefined,
  }));
}

async function loadLocationTree() {
  try {
    const resp = await locationStore.getLocationTree(undefined, 10);
    treeSelectData.value = buildTreeSelectData(resp.nodes ?? []);
  } catch (e) {
    console.error('Failed to load location tree:', e);
  }
}

const formState = ref({
  name: '',
  code: '',
  description: '',
  parentId: undefined as string | undefined,
  status: 'LOCATION_STATUS_ACTIVE',
  address: '',
  city: '',
  state: '',
  country: '',
  postalCode: '',
  contact: '',
  phone: '',
  email: '',
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('asset.page.location.create');
    case 'edit':
      return $t('asset.page.location.edit');
    default:
      return $t('asset.page.location.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await locationStore.createLocation({
        name: formState.value.name,
        code: formState.value.code || undefined,
        description: formState.value.description || undefined,
        parentId: formState.value.parentId,
        status: formState.value.status as any,
        address: formState.value.address || undefined,
        city: formState.value.city || undefined,
        state: formState.value.state || undefined,
        country: formState.value.country || undefined,
        postalCode: formState.value.postalCode || undefined,
        contact: formState.value.contact || undefined,
        phone: formState.value.phone || undefined,
        email: formState.value.email || undefined,
      });
      notification.success({
        message: $t('asset.page.location.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await locationStore.updateLocation(
        data.value.row.id,
        {
          name: formState.value.name,
          code: formState.value.code || undefined,
          description: formState.value.description || undefined,
          parentId: formState.value.parentId,
          status: formState.value.status as any,
          address: formState.value.address || undefined,
          city: formState.value.city || undefined,
          state: formState.value.state || undefined,
          country: formState.value.country || undefined,
          postalCode: formState.value.postalCode || undefined,
          contact: formState.value.contact || undefined,
          phone: formState.value.phone || undefined,
          email: formState.value.email || undefined,
        },
        [
          'name',
          'code',
          'description',
          'parentId',
          'status',
          'address',
          'city',
          'state',
          'country',
          'postalCode',
          'contact',
          'phone',
          'email',
        ],
      );
      notification.success({
        message: $t('asset.page.location.updateSuccess'),
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

function resetForm(parentId?: string) {
  formState.value = {
    name: '',
    code: '',
    description: '',
    parentId,
    status: 'LOCATION_STATUS_ACTIVE',
    address: '',
    city: '',
    state: '',
    country: '',
    postalCode: '',
    contact: '',
    phone: '',
    email: '',
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
        row?: Location;
        parentId?: string;
      };

      await loadLocationTree();

      if (data.value?.mode === 'create') {
        resetForm(data.value.parentId);
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name ?? '',
          code: data.value.row.code ?? '',
          description: data.value.row.description ?? '',
          parentId: data.value.row.parentId || undefined,
          status: data.value.row.status ?? 'LOCATION_STATUS_ACTIVE',
          address: data.value.row.address ?? '',
          city: data.value.row.city ?? '',
          state: data.value.row.state ?? '',
          country: data.value.row.country ?? '',
          postalCode: data.value.row.postalCode ?? '',
          contact: data.value.row.contact ?? '',
          phone: data.value.row.phone ?? '',
          email: data.value.row.email ?? '',
        };
      }
    }
  },
});

const location = computed(() => data.value?.row);
</script>

<template>
  <Drawer :title="title" :footer="false">
    <!-- View Mode -->
    <template v-if="location && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('asset.page.location.name')">
          {{ location.name }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.location.code')">
          {{ location.code || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.location.description')">
          {{ location.description || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.location.status')">
          <Tag :color="statusToColor(location.status)">
            {{ statusToName(location.status) }}
          </Tag>
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.location.childCount')">
          {{ location.childCount ?? 0 }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.location.assetCount')">
          {{ location.assetCount ?? 0 }}
        </DescriptionsItem>
      </Descriptions>

      <template
        v-if="
          location.address ||
          location.city ||
          location.state ||
          location.country
        "
      >
        <Divider orientation="left">{{
          $t('asset.page.location.sectionAddress')
        }}</Divider>
        <Descriptions :column="1" bordered size="small">
          <DescriptionsItem
            v-if="location.address"
            :label="$t('asset.page.location.address')"
          >
            {{ location.address }}
          </DescriptionsItem>
          <DescriptionsItem
            v-if="location.city"
            :label="$t('asset.page.location.city')"
          >
            {{ location.city }}
          </DescriptionsItem>
          <DescriptionsItem
            v-if="location.state"
            :label="$t('asset.page.location.state')"
          >
            {{ location.state }}
          </DescriptionsItem>
          <DescriptionsItem
            v-if="location.country"
            :label="$t('asset.page.location.country')"
          >
            {{ location.country }}
          </DescriptionsItem>
          <DescriptionsItem
            v-if="location.postalCode"
            :label="$t('asset.page.location.postalCode')"
          >
            {{ location.postalCode }}
          </DescriptionsItem>
        </Descriptions>
      </template>

      <template
        v-if="location.contact || location.phone || location.email"
      >
        <Divider orientation="left">{{
          $t('asset.page.location.sectionContact')
        }}</Divider>
        <Descriptions :column="1" bordered size="small">
          <DescriptionsItem
            v-if="location.contact"
            :label="$t('asset.page.location.contact')"
          >
            {{ location.contact }}
          </DescriptionsItem>
          <DescriptionsItem
            v-if="location.phone"
            :label="$t('asset.page.location.phone')"
          >
            {{ location.phone }}
          </DescriptionsItem>
          <DescriptionsItem
            v-if="location.email"
            :label="$t('asset.page.location.email')"
          >
            {{ location.email }}
          </DescriptionsItem>
        </Descriptions>
      </template>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('asset.page.location.name')"
          name="name"
          :rules="[{ required: true, message: $t('ui.formRules.required') }]"
        >
          <Input
            v-model:value="formState.name"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.location.code')" name="code">
          <Input
            v-model:value="formState.code"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="100"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.location.description')"
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
          :label="$t('asset.page.location.parentId')"
          name="parentId"
        >
          <TreeSelect
            v-model:value="formState.parentId"
            :tree-data="treeSelectData"
            :placeholder="$t('ui.placeholder.select')"
            allow-clear
            tree-default-expand-all
          />
        </FormItem>

        <FormItem :label="$t('asset.page.location.status')" name="status">
          <Select
            v-model:value="formState.status"
            :options="statusOptions"
          />
        </FormItem>

        <Divider orientation="left">{{
          $t('asset.page.location.sectionAddress')
        }}</Divider>

        <FormItem :label="$t('asset.page.location.address')" name="address">
          <Input
            v-model:value="formState.address"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="500"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.location.city')" name="city">
          <Input
            v-model:value="formState.city"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="100"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.location.state')" name="state">
          <Input
            v-model:value="formState.state"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="100"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.location.country')" name="country">
          <Input
            v-model:value="formState.country"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="100"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.location.postalCode')"
          name="postalCode"
        >
          <Input
            v-model:value="formState.postalCode"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="20"
          />
        </FormItem>

        <Divider orientation="left">{{
          $t('asset.page.location.sectionContact')
        }}</Divider>

        <FormItem :label="$t('asset.page.location.contact')" name="contact">
          <Input
            v-model:value="formState.contact"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.location.phone')" name="phone">
          <Input
            v-model:value="formState.phone"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="50"
          />
        </FormItem>

        <FormItem :label="$t('asset.page.location.email')" name="email">
          <Input
            v-model:value="formState.email"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="255"
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
