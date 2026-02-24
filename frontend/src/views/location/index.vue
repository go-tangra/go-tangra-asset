<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, ref, computed, onMounted } from 'vue';

import { Page, useVbenDrawer, type VbenFormProps } from 'shell/vben/common-ui';
import {
  LucideEye,
  LucideTrash,
  LucidePencil,
  LucidePlus,
} from 'shell/vben/icons';

import {
  notification,
  Space,
  Button,
  Tree,
  Card,
  Empty,
  Tag,
  Popconfirm,
} from 'ant-design-vue';
import type { TreeProps } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import type {
  Location,
  LocationTreeNode,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetLocationStore } from '../../stores/asset-location.state';

import LocationDrawer from './location-drawer.vue';

const locationStore = useAssetLocationStore();

const treeData = ref<TreeProps['treeData']>([]);
const selectedLocationId = ref<string | undefined>(undefined);
const selectedLocation = ref<Location | null>(null);
const treeLoading = ref(false);
const expandedKeys = ref<(number | string)[]>([]);

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

interface TreeNode {
  key: string;
  title: string;
  children?: TreeNode[];
  location: Location | undefined;
  statusColor: string;
}

function buildTreeData(nodes: LocationTreeNode[]): TreeNode[] {
  return nodes.map((node) => ({
    key: node.location?.id ?? '',
    title: node.location?.name ?? '',
    children: node.children ? buildTreeData(node.children) : undefined,
    location: node.location,
    statusColor: statusToColor(node.location?.status),
  }));
}

async function loadLocationTree() {
  treeLoading.value = true;
  try {
    const resp = await locationStore.getLocationTree(undefined, 10);
    treeData.value = buildTreeData(resp.nodes ?? []);
    if (treeData.value.length > 0) {
      expandedKeys.value = treeData.value.map((n) => n.key);
    }
  } catch (e) {
    console.error('Failed to load location tree:', e);
  } finally {
    treeLoading.value = false;
  }
}

function handleTreeSelect(keys: (number | string)[], info: { node: any }) {
  if (keys.length > 0) {
    selectedLocationId.value = String(keys[0]);
    selectedLocation.value = info.node.location ?? null;
    gridApi.reload();
  } else {
    selectedLocationId.value = undefined;
    selectedLocation.value = null;
  }
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'query',
      label: $t('asset.page.location.name'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Location> = {
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
        const resp = await locationStore.listLocations(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            parentId: selectedLocationId.value,
            query: formValues?.query,
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
      title: $t('asset.page.location.name'),
      field: 'name',
      minWidth: 150,
    },
    {
      title: $t('asset.page.location.code'),
      field: 'code',
      width: 120,
    },
    {
      title: $t('asset.page.location.status'),
      field: 'status',
      width: 130,
      slots: { default: 'status' },
    },
    {
      title: $t('asset.page.location.address'),
      field: 'address',
      minWidth: 200,
    },
    {
      title: $t('asset.page.location.childCount'),
      field: 'childCount',
      width: 100,
    },
    {
      title: $t('asset.page.location.assetCount'),
      field: 'assetCount',
      width: 100,
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 180,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [LocationDrawerComponent, locationDrawerApi] = useVbenDrawer({
  connectedComponent: LocationDrawer,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      loadLocationTree();
      gridApi.query();
    }
  },
});

function openDrawer(
  row: Location,
  mode: 'create' | 'edit' | 'view',
  parentId?: string,
) {
  locationDrawerApi.setData({ row, mode, parentId });
  locationDrawerApi.open();
}

function handleView(row: Location) {
  openDrawer(row, 'view');
}

function handleEdit(row: Location) {
  openDrawer(row, 'edit');
}

function handleCreate() {
  openDrawer({} as Location, 'create', selectedLocationId.value);
}

function handleCreateChild(row: Location) {
  openDrawer({} as Location, 'create', row.id);
}

async function handleDelete(row: Location) {
  if (!row.id) return;
  try {
    await locationStore.deleteLocation(row.id);
    notification.success({
      message: $t('asset.page.location.deleteSuccess'),
    });
    await loadLocationTree();
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

function handleEditFromTree(location: Location | undefined) {
  if (!location) return;
  openDrawer(location, 'edit');
}

async function handleDeleteFromTree(location: Location | undefined) {
  if (!location?.id) return;
  try {
    await locationStore.deleteLocation(location.id);
    notification.success({
      message: $t('asset.page.location.deleteSuccess'),
    });
    if (selectedLocationId.value === location.id) {
      selectedLocationId.value = undefined;
      selectedLocation.value = null;
    }
    await loadLocationTree();
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

onMounted(() => {
  loadLocationTree();
});
</script>

<template>
  <Page auto-content-height>
    <div class="flex h-full gap-4">
      <!-- Left: Location Tree -->
      <Card
        class="w-[300px] shrink-0"
        :title="$t('asset.page.location.tree')"
        :loading="treeLoading"
      >
        <template #extra>
          <Button type="link" size="small" @click="handleCreate">
            <template #icon>
              <component :is="LucidePlus" class="size-4" />
            </template>
          </Button>
        </template>
        <div v-if="treeData && treeData.length > 0">
          <Tree
            v-model:expandedKeys="expandedKeys"
            :tree-data="treeData"
            :selectable="true"
            :default-expand-all="true"
            @select="handleTreeSelect"
          >
            <template #title="{ title, location, statusColor }">
              <div
                class="tree-node-title group flex items-center justify-between pr-1"
              >
                <span class="flex items-center gap-1 truncate flex-1">
                  <span
                    class="inline-block h-2 w-2 rounded-full"
                    :style="{ backgroundColor: statusColor }"
                  />
                  {{ title }}
                </span>
                <span
                  class="tree-actions flex items-center"
                >
                  <Button
                    type="text"
                    size="small"
                    class="!px-1"
                    :title="$t('ui.button.edit')"
                    @click.stop="handleEditFromTree(location)"
                  >
                    <template #icon>
                      <LucidePencil class="size-3" />
                    </template>
                  </Button>
                  <Popconfirm
                    :title="$t('asset.page.location.confirmDelete')"
                    :ok-text="$t('ui.button.ok')"
                    :cancel-text="$t('ui.button.cancel')"
                    @confirm.stop="handleDeleteFromTree(location)"
                  >
                    <Button
                      type="text"
                      size="small"
                      danger
                      class="!px-1"
                      :title="$t('ui.button.delete', { moduleName: '' })"
                      @click.stop
                    >
                      <template #icon>
                        <LucideTrash class="size-3" />
                      </template>
                    </Button>
                  </Popconfirm>
                </span>
              </div>
            </template>
          </Tree>
        </div>
        <Empty v-else :description="$t('ui.table.empty')" />
      </Card>

      <!-- Right: Location List -->
      <div class="min-w-0 flex-1">
        <Grid
          :table-title="
            selectedLocation
              ? selectedLocation.name
              : $t('asset.page.location.title')
          "
        >
          <template #toolbar-tools>
            <Button class="mr-2" type="primary" @click="handleCreate">
              {{ $t('asset.page.location.create') }}
            </Button>
          </template>
          <template #status="{ row }">
            <Tag :color="statusToColor(row.status)">
              {{ statusToName(row.status) }}
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
                :icon="h(LucidePlus)"
                :title="$t('asset.page.location.createChild')"
                @click.stop="handleCreateChild(row)"
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
                :title="$t('asset.page.location.confirmDelete')"
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
      </div>
    </div>

    <LocationDrawerComponent />
  </Page>
</template>

<style scoped>
.tree-actions {
  opacity: 0;
  transition: opacity 0.2s;
}
.group:hover > .tree-actions {
  opacity: 1;
}
</style>
