<script lang="ts" setup>
import type { VxeGridProps } from 'shell/adapter/vxe-table';

import { h, ref, computed, onMounted } from 'vue';

import { Page, useVbenModal, type VbenFormProps } from 'shell/vben/common-ui';
import {
  LucideEye,
  LucideTrash,
  LucidePencil,
  LucideLink,
  LucideBan,
  LucideShieldCheck,
  LucideRefreshCw,
} from 'shell/vben/icons';

import { notification, Space, Button, Tag, Modal, Select, Input, InputNumber } from 'ant-design-vue';

import { useVbenVxeGrid } from 'shell/adapter/vxe-table';
import type { Asset } from '../../api/services';
import {
  CategoryService,
  UserService,
  LocationService,
  SupplierService,
  InsurancePolicyService,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetAssetStore } from '../../stores/asset-asset.state';
import { useAssetInsuranceStore } from '../../stores/asset-insurance.state';

import AssetModal from './asset-modal.vue';
import InventorySyncModal from './inventory-sync-modal.vue';

const assetStore = useAssetAssetStore();
const insuranceStore = useAssetInsuranceStore();

const categoryMap = ref<Map<string, string>>(new Map());
const userMap = ref<Map<number, string>>(new Map());
const supplierMap = ref<Map<string, string>>(new Map());

interface SelectOption {
  value: string | number;
  label: string;
}

const categoryOptions = ref<SelectOption[]>([]);
const userOptions = ref<SelectOption[]>([]);
const supplierOptions = ref<SelectOption[]>([]);
const locationOptions = ref<SelectOption[]>([]);
const policyOptions = ref<SelectOption[]>([]);
const insuredAssetIds = ref<Set<string>>(new Set());

async function loadRelatedData() {
  try {
    const [catResp, usrResp, supResp, locResp, polResp] = await Promise.all([
      CategoryService.list({ noPaging: true }),
      UserService.list(),
      SupplierService.list({ noPaging: true }),
      LocationService.list({ noPaging: true }),
      InsurancePolicyService.list({ noPaging: true }),
    ]);
    const catMap = new Map<string, string>();
    categoryOptions.value = (catResp.items ?? []).map((c) => {
      catMap.set(c.id, c.name);
      return { value: c.id, label: c.name };
    });
    categoryMap.value = catMap;

    const usrMap = new Map<number, string>();
    userOptions.value = (usrResp.items ?? []).map((u) => {
      const name = u.realname || u.username;
      usrMap.set(u.id, name);
      return { value: u.id, label: name };
    });
    userMap.value = usrMap;

    const supMap = new Map<string, string>();
    supplierOptions.value = (supResp.items ?? []).map((s) => {
      supMap.set(s.id, s.name);
      return { value: s.id, label: s.name };
    });
    supplierMap.value = supMap;

    locationOptions.value = (locResp.items ?? []).map((l) => ({
      value: l.id,
      label: l.name,
    }));

    const policies = polResp.items ?? [];
    policyOptions.value = policies.map((p) => ({
      value: p.id,
      label: p.policyNumber ? `${p.name} (${p.policyNumber})` : p.name,
    }));

    // Load insured asset IDs from all policies
    const ids = new Set<string>();
    const assetLists = await Promise.all(
      policies.map((p) => InsurancePolicyService.listAssets(p.id)),
    );
    for (const resp of assetLists) {
      for (const pa of resp.items ?? []) {
        if (pa.assetId) ids.add(pa.assetId);
      }
    }
    insuredAssetIds.value = ids;
  } catch (e) {
    console.error('Failed to load related data:', e);
  }
}

const statusOptions = computed(() => [
  {
    value: 'ASSET_STATUS_DEPLOYABLE',
    label: $t('asset.enum.assetStatus.deployable'),
  },
  {
    value: 'ASSET_STATUS_ASSIGNED',
    label: $t('asset.enum.assetStatus.assigned'),
  },
  {
    value: 'ASSET_STATUS_BROKEN',
    label: $t('asset.enum.assetStatus.broken'),
  },
  {
    value: 'ASSET_STATUS_ARCHIVED',
    label: $t('asset.enum.assetStatus.archived'),
  },
]);

function statusToColor(status: string | undefined) {
  switch (status) {
    case 'ASSET_STATUS_DEPLOYABLE':
      return '#52C41A';
    case 'ASSET_STATUS_ASSIGNED':
      return '#1890FF';
    case 'ASSET_STATUS_BROKEN':
      return '#FF4D4F';
    case 'ASSET_STATUS_ARCHIVED':
      return '#8C8C8C';
    default:
      return '#C9CDD4';
  }
}

function statusToName(status: string | undefined) {
  const option = statusOptions.value.find((o) => o.value === status);
  return option?.label ?? status ?? '';
}

function getCategoryName(id: string | undefined) {
  if (!id) return '-';
  return categoryMap.value.get(id) ?? id;
}

function getUserName(id: number | undefined) {
  if (!id) return '-';
  return userMap.value.get(id) ?? `User #${id}`;
}

function getSupplierName(id: string | undefined) {
  if (!id) return '-';
  return supplierMap.value.get(id) ?? id;
}

function calculateCurrentValue(row: Asset): string {
  const cost = row.purchaseCost;
  const usefulLife = row.usefulLifeYears;
  const salvage = row.salvageValue ?? 0;
  if (cost == null || !usefulLife || usefulLife <= 0) return '-';

  const purchaseDate = row.purchaseDate ? new Date(row.purchaseDate) : null;
  const now = new Date();
  const yearsElapsed = purchaseDate
    ? Math.floor(
        (now.getTime() - purchaseDate.getTime()) / (365.25 * 24 * 60 * 60 * 1000),
      )
    : 0;
  if (yearsElapsed <= 0) return cost.toFixed(2);

  const rate = row.depreciationRate ?? 2 / usefulLife;
  let bookValue = cost;
  const years = Math.min(yearsElapsed, usefulLife);
  for (let i = 0; i < years; i++) {
    let depreciation = bookValue * rate;
    if (bookValue - depreciation < salvage) {
      depreciation = bookValue - salvage;
    }
    bookValue -= depreciation;
    if (bookValue <= salvage) {
      bookValue = salvage;
      break;
    }
  }

  return bookValue.toFixed(2);
}

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
      label: $t('asset.page.asset.status'),
      componentProps: {
        options: statusOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'categoryId',
      label: $t('asset.page.asset.categoryId'),
      componentProps: {
        options: categoryOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'supplierId',
      label: $t('asset.page.asset.supplierId'),
      componentProps: {
        options: supplierOptions,
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps<Asset> = {
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
        const resp = await assetStore.listAssets(
          { page: page.currentPage, pageSize: page.pageSize },
          {
            query: formValues?.query,
            status: formValues?.status,
            categoryId: formValues?.categoryId,
            supplierId: formValues?.supplierId,
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
      title: $t('asset.page.asset.assetTag'),
      field: 'assetTag',
      width: 140,
    },
    {
      title: $t('asset.page.asset.status'),
      field: 'status',
      width: 120,
      slots: { default: 'status' },
    },
    {
      title: $t('asset.page.asset.modelName'),
      field: 'modelName',
      width: 140,
    },
    {
      title: $t('asset.page.asset.categoryId'),
      field: 'categoryId',
      width: 140,
      slots: { default: 'categoryName' },
    },
    {
      title: $t('asset.page.asset.supplierId'),
      field: 'supplierId',
      width: 140,
      slots: { default: 'supplierName' },
    },
    {
      title: $t('asset.page.asset.userId'),
      field: 'userId',
      width: 140,
      slots: { default: 'userName' },
    },
    {
      title: $t('asset.page.asset.purchaseCost'),
      field: 'purchaseCost',
      width: 120,
    },
    {
      title: $t('asset.page.asset.currentValue'),
      field: 'currentValue',
      width: 130,
      slots: { default: 'currentValue' },
    },
    {
      title: $t('ui.table.action'),
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 230,
    },
  ],
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions, formOptions });

const [AssetModalComponent, assetModalApi] = useVbenModal({
  connectedComponent: AssetModal,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

const [InventorySyncModalComponent, inventorySyncModalApi] = useVbenModal({
  connectedComponent: InventorySyncModal,
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.query();
    }
  },
});

function handleInventorySync() {
  inventorySyncModalApi.open();
}

function openModal(row: Asset, mode: 'create' | 'edit' | 'view') {
  assetModalApi.setData({ row, mode });
  assetModalApi.open();
}

function handleView(row: Asset) {
  openModal(row, 'view');
}

function handleEdit(row: Asset) {
  openModal(row, 'edit');
}

function handleCreate() {
  openModal({} as Asset, 'create');
}

async function handleDelete(row: Asset) {
  if (!row.id) return;
  try {
    await assetStore.deleteAsset(row.id);
    notification.success({
      message: $t('asset.page.asset.deleteSuccess'),
    });
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

// Assign modal
const assignModalVisible = ref(false);
const assignTarget = ref<Asset | null>(null);
const assignUserId = ref<number | undefined>(undefined);
const assignNotes = ref('');

function handleAssign(row: Asset) {
  assignTarget.value = row;
  assignUserId.value = undefined;
  assignNotes.value = '';
  assignModalVisible.value = true;
}

async function confirmAssign() {
  if (!assignTarget.value?.id || !assignUserId.value) return;
  try {
    await assetStore.assignAsset(
      assignTarget.value.id,
      assignUserId.value,
      assignNotes.value || undefined,
    );
    notification.success({
      message: $t('asset.page.asset.assignSuccess'),
    });
    assignModalVisible.value = false;
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.update_failed') });
  }
}

// Unassign modal
const unassignModalVisible = ref(false);
const unassignTarget = ref<Asset | null>(null);
const unassignLocationId = ref<string | undefined>(undefined);
const unassignNotes = ref('');

function handleUnassign(row: Asset) {
  unassignTarget.value = row;
  unassignLocationId.value = undefined;
  unassignNotes.value = '';
  unassignModalVisible.value = true;
}

async function confirmUnassign() {
  if (!unassignTarget.value?.id) return;
  try {
    await assetStore.unassignAsset(
      unassignTarget.value.id,
      unassignLocationId.value,
      unassignNotes.value || undefined,
    );
    notification.success({
      message: $t('asset.page.asset.unassignSuccess'),
    });
    unassignModalVisible.value = false;
    await gridApi.query();
  } catch {
    notification.error({ message: $t('ui.notification.update_failed') });
  }
}

// Add to insurance policy modal
const addToPolicyModalVisible = ref(false);
const addToPolicyTarget = ref<Asset | null>(null);
const addToPolicyId = ref<string | undefined>(undefined);
const addToPolicyCoveredValue = ref<number | undefined>(undefined);
const addToPolicyNotes = ref('');

function handleAddToPolicy(row: Asset) {
  addToPolicyTarget.value = row;
  addToPolicyId.value = undefined;
  addToPolicyCoveredValue.value = undefined;
  addToPolicyNotes.value = '';
  addToPolicyModalVisible.value = true;
}

async function confirmAddToPolicy() {
  if (!addToPolicyTarget.value?.id || !addToPolicyId.value) return;
  try {
    await insuranceStore.addAssetToPolicy(
      addToPolicyId.value,
      addToPolicyTarget.value.id,
      addToPolicyCoveredValue.value,
      addToPolicyNotes.value || undefined,
    );
    insuredAssetIds.value.add(addToPolicyTarget.value.id);
    insuredAssetIds.value = new Set(insuredAssetIds.value);
    notification.success({
      message: $t('asset.page.insurance.addAssetSuccess'),
    });
    addToPolicyModalVisible.value = false;
  } catch {
    notification.error({ message: $t('ui.notification.update_failed') });
  }
}

onMounted(() => {
  loadRelatedData();
});
</script>

<template>
  <Page auto-content-height>
    <Grid :table-title="$t('asset.page.asset.title')">
      <template #toolbar-tools>
        <Button class="mr-2" @click="handleInventorySync">
          <template #icon><LucideRefreshCw class="size-4" /></template>
          {{ $t('asset.page.asset.inventory.syncButton') }}
        </Button>
        <Button class="mr-2" type="primary" @click="handleCreate">
          {{ $t('asset.page.asset.create') }}
        </Button>
      </template>
      <template #status="{ row }">
        <Tag :color="statusToColor(row.status)">
          {{ statusToName(row.status) }}
        </Tag>
      </template>
      <template #categoryName="{ row }">
        {{ getCategoryName(row.categoryId) }}
      </template>
      <template #supplierName="{ row }">
        {{ getSupplierName(row.supplierId) }}
      </template>
      <template #userName="{ row }">
        {{ getUserName(row.userId) }}
      </template>
      <template #currentValue="{ row }">
        {{ calculateCurrentValue(row) }}
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
            v-if="row.status === 'ASSET_STATUS_DEPLOYABLE'"
            type="link"
            size="small"
            :icon="h(LucideLink)"
            :title="$t('asset.page.asset.assign')"
            @click.stop="handleAssign(row)"
          />
          <Button
            v-if="row.status === 'ASSET_STATUS_ASSIGNED'"
            type="link"
            size="small"
            :icon="h(LucideBan)"
            :title="$t('asset.page.asset.unassign')"
            @click.stop="handleUnassign(row)"
          />
          <Button
            type="link"
            size="small"
            :icon="h(LucideShieldCheck)"
            :title="$t('asset.page.asset.addToPolicy')"
            :disabled="insuredAssetIds.has(row.id)"
            @click.stop="handleAddToPolicy(row)"
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
            :title="$t('asset.page.asset.confirmDelete')"
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

    <AssetModalComponent />
    <InventorySyncModalComponent />

    <!-- Assign Modal -->
    <Modal
      v-model:open="assignModalVisible"
      :title="$t('asset.page.asset.assignUser')"
      :ok-text="$t('asset.page.asset.assign')"
      :cancel-text="$t('ui.button.cancel')"
      @ok="confirmAssign"
    >
      <div class="mb-4">
        <label class="mb-1 block font-medium">{{
          $t('asset.page.asset.userId')
        }}</label>
        <Select
          v-model:value="assignUserId"
          :options="userOptions"
          :placeholder="$t('ui.placeholder.select')"
          show-search
          :filter-option="
            (input: string, option: any) =>
              option.label.toLowerCase().includes(input.toLowerCase())
          "
          style="width: 100%"
        />
      </div>
      <div>
        <label class="mb-1 block font-medium">{{
          $t('asset.page.asset.assignNotes')
        }}</label>
        <Input.TextArea
          v-model:value="assignNotes"
          :rows="3"
          :placeholder="$t('ui.placeholder.input')"
        />
      </div>
    </Modal>

    <!-- Unassign Modal -->
    <Modal
      v-model:open="unassignModalVisible"
      :title="$t('asset.page.asset.unassign')"
      :ok-text="$t('asset.page.asset.unassign')"
      :cancel-text="$t('ui.button.cancel')"
      @ok="confirmUnassign"
    >
      <div class="mb-4">
        <label class="mb-1 block font-medium">{{
          $t('asset.page.asset.returnLocation')
        }}</label>
        <Select
          v-model:value="unassignLocationId"
          :options="locationOptions"
          :placeholder="$t('ui.placeholder.select')"
          allow-clear
          show-search
          :filter-option="
            (input: string, option: any) =>
              option.label.toLowerCase().includes(input.toLowerCase())
          "
          style="width: 100%"
        />
      </div>
      <div>
        <label class="mb-1 block font-medium">{{
          $t('asset.page.asset.unassignNotes')
        }}</label>
        <Input.TextArea
          v-model:value="unassignNotes"
          :rows="3"
          :placeholder="$t('ui.placeholder.input')"
        />
      </div>
    </Modal>

    <!-- Add to Insurance Policy Modal -->
    <Modal
      v-model:open="addToPolicyModalVisible"
      :title="$t('asset.page.asset.addToPolicy')"
      :ok-text="$t('asset.page.insurance.addAsset')"
      :cancel-text="$t('ui.button.cancel')"
      @ok="confirmAddToPolicy"
    >
      <div class="mb-4">
        <label class="mb-1 block font-medium">{{
          $t('asset.page.asset.selectPolicy')
        }}</label>
        <Select
          v-model:value="addToPolicyId"
          :options="policyOptions"
          :placeholder="$t('ui.placeholder.select')"
          show-search
          :filter-option="
            (input: string, option: any) =>
              option.label.toLowerCase().includes(input.toLowerCase())
          "
          style="width: 100%"
        />
      </div>
      <div class="mb-4">
        <label class="mb-1 block font-medium">{{
          $t('asset.page.insurance.coveredValue')
        }}</label>
        <InputNumber
          v-model:value="addToPolicyCoveredValue"
          :min="0"
          :precision="2"
          style="width: 100%"
        />
      </div>
      <div>
        <label class="mb-1 block font-medium">{{
          $t('asset.page.insurance.notes')
        }}</label>
        <Input.TextArea
          v-model:value="addToPolicyNotes"
          :rows="3"
          :placeholder="$t('ui.placeholder.input')"
        />
      </div>
    </Modal>
  </Page>
</template>
