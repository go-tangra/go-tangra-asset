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
  TreeSelect,
  Descriptions,
  DescriptionsItem,
} from 'ant-design-vue';

import type {
  Category,
  CategoryTreeNode,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetCategoryStore } from '../../stores/asset-category.state';

const categoryStore = useAssetCategoryStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: Category;
  parentId?: string;
}>();
const loading = ref(false);

interface TreeSelectNode {
  value: string;
  title: string;
  children?: TreeSelectNode[];
}

const treeSelectData = ref<TreeSelectNode[]>([]);

function buildTreeSelectData(nodes: CategoryTreeNode[]): TreeSelectNode[] {
  return nodes.map((node) => ({
    value: node.category?.id ?? '',
    title: node.category?.name ?? '',
    children: node.children ? buildTreeSelectData(node.children) : undefined,
  }));
}

async function loadCategoryTree() {
  try {
    const resp = await categoryStore.getCategoryTree(undefined, 10);
    treeSelectData.value = buildTreeSelectData(resp.nodes ?? []);
  } catch (e) {
    console.error('Failed to load category tree:', e);
  }
}

const formState = ref({
  name: '',
  description: '',
  parentId: undefined as string | undefined,
  icon: '',
  metadata: '',
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('asset.page.category.create');
    case 'edit':
      return $t('asset.page.category.edit');
    default:
      return $t('asset.page.category.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await categoryStore.createCategory({
        name: formState.value.name,
        description: formState.value.description || undefined,
        parentId: formState.value.parentId,
        icon: formState.value.icon || undefined,
        metadata: formState.value.metadata ? JSON.parse(formState.value.metadata) : undefined,
      });
      notification.success({
        message: $t('asset.page.category.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await categoryStore.updateCategory(
        data.value.row.id,
        {
          name: formState.value.name,
          description: formState.value.description || undefined,
          parentId: formState.value.parentId,
          icon: formState.value.icon || undefined,
          metadata: formState.value.metadata ? JSON.parse(formState.value.metadata) : undefined,
        },
        ['name', 'description', 'parentId', 'icon', 'metadata'],
      );
      notification.success({
        message: $t('asset.page.category.updateSuccess'),
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
    description: '',
    parentId,
    icon: '',
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
        row?: Category;
        parentId?: string;
      };

      await loadCategoryTree();

      if (data.value?.mode === 'create') {
        resetForm(data.value.parentId);
      } else if (data.value?.row) {
        formState.value = {
          name: data.value.row.name ?? '',
          description: data.value.row.description ?? '',
          parentId: data.value.row.parentId || undefined,
          icon: data.value.row.icon ?? '',
          metadata: data.value.row.metadata ? JSON.stringify(data.value.row.metadata, null, 2) : '',
        };
      }
    }
  },
});

const category = computed(() => data.value?.row);
</script>

<template>
  <Drawer :title="title" :footer="false">
    <!-- View Mode -->
    <template v-if="category && isViewMode">
      <Descriptions :column="1" bordered size="small">
        <DescriptionsItem :label="$t('asset.page.category.name')">
          {{ category.name }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.category.description')">
          {{ category.description || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.category.icon')">
          {{ category.icon || '-' }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.category.assetCount')">
          {{ category.assetCount ?? 0 }}
        </DescriptionsItem>
        <DescriptionsItem :label="$t('asset.page.category.childCount')">
          {{ category.childCount ?? 0 }}
        </DescriptionsItem>
        <DescriptionsItem
          v-if="category.metadata && Object.keys(category.metadata).length > 0"
          :label="$t('asset.page.category.metadata')"
        >
          <pre class="m-0 whitespace-pre-wrap text-xs">{{ JSON.stringify(category.metadata, null, 2) }}</pre>
        </DescriptionsItem>
      </Descriptions>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <FormItem
          :label="$t('asset.page.category.name')"
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
          :label="$t('asset.page.category.description')"
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
          :label="$t('asset.page.category.parentId')"
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

        <FormItem :label="$t('asset.page.category.icon')" name="icon">
          <Input
            v-model:value="formState.icon"
            :placeholder="$t('ui.placeholder.input')"
            :maxlength="100"
          />
        </FormItem>

        <FormItem
          :label="$t('asset.page.category.metadata')"
          name="metadata"
        >
          <Textarea
            v-model:value="formState.metadata"
            :rows="4"
            placeholder='{"key": "value"}'
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
