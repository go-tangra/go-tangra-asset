<script lang="ts" setup>
import { ref, computed } from 'vue';

import { useVbenModal } from 'shell/vben/common-ui';
import {
  LucideUpload,
  LucideFile,
  LucideTrash,
  LucideDownload,
} from 'shell/vben/icons';

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
  Table,
  Popconfirm,
  Image,
  Spin,
  Tabs,
  TabPane,
  DatePicker,
} from 'ant-design-vue';

import type {
  Asset,
  AssetAssignment,
  AssetDocument,
} from '../../api/services';
import {
  CategoryService,
  SupplierService,
  LocationService,
  EmployeeService,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetAssetStore } from '../../stores/asset-asset.state';

const assetStore = useAssetAssetStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: Asset;
}>();
const loading = ref(false);
const activeTab = ref('general');

interface SelectOption {
  value: string;
  label: string;
}

const categoryOptions = ref<SelectOption[]>([]);
const supplierOptions = ref<SelectOption[]>([]);
const locationOptions = ref<SelectOption[]>([]);
const employeeOptions = ref<SelectOption[]>([]);

const assignmentHistory = ref<AssetAssignment[]>([]);

// Photo state
const photoUrl = ref<string | undefined>(undefined);
const photoLoading = ref(false);
const photoInputRef = ref<HTMLInputElement | null>(null);
const isPhotoDragOver = ref(false);

// Document state
const documents = ref<AssetDocument[]>([]);
const documentsLoading = ref(false);
const docInputRef = ref<HTMLInputElement | null>(null);
const isDocDragOver = ref(false);
const docDescription = ref('');

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      const base64 = result.split(',')[1];
      resolve(base64 ?? '');
    };
    reader.onerror = () =>
      reject(new Error(`Failed to read file: ${file.name}`));
    reader.readAsDataURL(file);
  });
}

function formatFileSize(bytes: number | undefined): string {
  if (!bytes) return '-';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// Photo methods
function triggerPhotoInput() {
  setTimeout(() => photoInputRef.value?.click(), 0);
}

async function handlePhotoFile(file: File) {
  const assetId = data.value?.row?.id;
  if (!assetId) return;

  photoLoading.value = true;
  try {
    const base64 = await fileToBase64(file);
    const resp = await assetStore.uploadPhoto(assetId, base64, file.name);
    photoUrl.value = resp.asset?.photoKey
      ? `/admin/v1/modules/asset/v1/assets/${assetId}/photo?t=${Date.now()}`
      : undefined;
    notification.success({
      message: $t('asset.page.asset.photoUploadSuccess'),
    });
  } catch {
    notification.error({ message: $t('ui.notification.upload_failed') });
  } finally {
    photoLoading.value = false;
  }
}

function handlePhotoInputChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (file) handlePhotoFile(file);
  input.value = '';
}

function handlePhotoDrop(e: DragEvent) {
  e.preventDefault();
  isPhotoDragOver.value = false;
  const file = e.dataTransfer?.files[0];
  if (file && file.type.startsWith('image/')) handlePhotoFile(file);
}

async function handleDeletePhoto() {
  const assetId = data.value?.row?.id;
  if (!assetId) return;

  photoLoading.value = true;
  try {
    await assetStore.deletePhoto(assetId);
    photoUrl.value = undefined;
    notification.success({
      message: $t('asset.page.asset.photoDeleteSuccess'),
    });
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  } finally {
    photoLoading.value = false;
  }
}

// Document methods
async function loadDocuments(assetId: string) {
  documentsLoading.value = true;
  try {
    const resp = await assetStore.listDocuments(assetId);
    documents.value = resp.items ?? [];
  } catch (e) {
    console.error('Failed to load documents:', e);
    documents.value = [];
  } finally {
    documentsLoading.value = false;
  }
}

function triggerDocInput() {
  setTimeout(() => docInputRef.value?.click(), 0);
}

async function handleDocFile(file: File) {
  const assetId = data.value?.row?.id;
  if (!assetId) return;

  documentsLoading.value = true;
  try {
    const base64 = await fileToBase64(file);
    await assetStore.uploadDocument(
      assetId,
      file.name,
      base64,
      docDescription.value || undefined,
    );
    docDescription.value = '';
    notification.success({
      message: $t('asset.page.asset.documentUploadSuccess'),
    });
    await loadDocuments(assetId);
  } catch {
    notification.error({ message: $t('ui.notification.upload_failed') });
  } finally {
    documentsLoading.value = false;
  }
}

function handleDocInputChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (file) handleDocFile(file);
  input.value = '';
}

function handleDocDrop(e: DragEvent) {
  e.preventDefault();
  isDocDragOver.value = false;
  const file = e.dataTransfer?.files[0];
  if (file) handleDocFile(file);
}

async function handleDeleteDocument(docId: string) {
  const assetId = data.value?.row?.id;
  if (!assetId) return;

  try {
    await assetStore.deleteDocument(assetId, docId);
    notification.success({
      message: $t('asset.page.asset.documentDeleteSuccess'),
    });
    await loadDocuments(assetId);
  } catch {
    notification.error({ message: $t('ui.notification.delete_failed') });
  }
}

async function handleDownloadDocument(doc: AssetDocument) {
  const assetId = data.value?.row?.id;
  if (!assetId) return;

  try {
    const resp = await assetStore.downloadDocument(assetId, doc.id);
    const byteCharacters = atob(resp.content);
    const byteArray = new Uint8Array(byteCharacters.length);
    for (let i = 0; i < byteCharacters.length; i++) {
      byteArray[i] = byteCharacters.charCodeAt(i);
    }
    const blob = new Blob([byteArray], {
      type: resp.mimeType || 'application/octet-stream',
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = resp.fileName || doc.fileName;
    a.click();
    URL.revokeObjectURL(url);
  } catch {
    notification.error({ message: $t('ui.notification.download_failed') });
  }
}

async function loadOptions() {
  try {
    const [catResp, supResp, locResp, empResp] = await Promise.all([
      CategoryService.list({ noPaging: true }),
      SupplierService.list({ noPaging: true }),
      LocationService.list({ noPaging: true }),
      EmployeeService.list({ noPaging: true }),
    ]);
    categoryOptions.value = (catResp.items ?? []).map((c) => ({
      value: c.id,
      label: c.name,
    }));
    supplierOptions.value = (supResp.items ?? []).map((s) => ({
      value: s.id,
      label: s.name,
    }));
    locationOptions.value = (locResp.items ?? []).map((l) => ({
      value: l.id,
      label: l.name,
    }));
    employeeOptions.value = (empResp.items ?? []).map((e) => ({
      value: e.id,
      label: `${e.firstName} ${e.lastName}`,
    }));
  } catch (e) {
    console.error('Failed to load options:', e);
  }
}

async function loadAssignmentHistory(assetId: string) {
  try {
    const resp = await assetStore.getAssignmentHistory(assetId);
    assignmentHistory.value = resp.items ?? [];
  } catch (e) {
    console.error('Failed to load assignment history:', e);
    assignmentHistory.value = [];
  }
}

function getOptionLabel(options: SelectOption[], id: string | undefined) {
  if (!id) return '-';
  return options.find((o) => o.value === id)?.label ?? id;
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

function actionToName(action: string | undefined) {
  switch (action) {
    case 'ASSIGNMENT_ACTION_ASSIGNED':
      return $t('asset.enum.assignmentAction.assigned');
    case 'ASSIGNMENT_ACTION_UNASSIGNED':
      return $t('asset.enum.assignmentAction.unassigned');
    case 'ASSIGNMENT_ACTION_TRANSFERRED':
      return $t('asset.enum.assignmentAction.transferred');
    default:
      return action ?? '';
  }
}

const formState = ref({
  serial: '',
  modelName: '',
  modelNumber: '',
  categoryId: undefined as string | undefined,
  supplierId: undefined as string | undefined,
  locationId: undefined as string | undefined,
  status: 'ASSET_STATUS_DEPLOYABLE',
  warrantyMonths: undefined as number | undefined,
  purchaseDate: '',
  orderNumber: '',
  purchaseCost: undefined as number | undefined,
  notes: '',
  salvageValue: undefined as number | undefined,
  usefulLifeYears: undefined as number | undefined,
  depreciationRate: 0.40,
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('asset.page.asset.create');
    case 'edit':
      return $t('asset.page.asset.edit');
    default:
      return $t('asset.page.asset.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await assetStore.createAsset({
        serial: formState.value.serial || undefined,
        modelName: formState.value.modelName || undefined,
        modelNumber: formState.value.modelNumber || undefined,
        categoryId: formState.value.categoryId,
        supplierId: formState.value.supplierId,
        locationId: formState.value.locationId,
        status: formState.value.status as any,
        warrantyMonths: formState.value.warrantyMonths,
        purchaseDate: formState.value.purchaseDate ? `${formState.value.purchaseDate}T00:00:00Z` : undefined,
        orderNumber: formState.value.orderNumber || undefined,
        purchaseCost: formState.value.purchaseCost,
        notes: formState.value.notes || undefined,
        salvageValue: formState.value.salvageValue,
        usefulLifeYears: formState.value.usefulLifeYears,
        depreciationRate: formState.value.depreciationRate,
      });
      notification.success({
        message: $t('asset.page.asset.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await assetStore.updateAsset(
        data.value.row.id,
        {
          serial: formState.value.serial || undefined,
          modelName: formState.value.modelName || undefined,
          modelNumber: formState.value.modelNumber || undefined,
          categoryId: formState.value.categoryId,
          supplierId: formState.value.supplierId,
          locationId: formState.value.locationId,
          status: formState.value.status as any,
          warrantyMonths: formState.value.warrantyMonths,
          purchaseDate: formState.value.purchaseDate ? `${formState.value.purchaseDate}T00:00:00Z` : undefined,
          orderNumber: formState.value.orderNumber || undefined,
          purchaseCost: formState.value.purchaseCost,
          notes: formState.value.notes || undefined,
          salvageValue: formState.value.salvageValue,
          usefulLifeYears: formState.value.usefulLifeYears,
          depreciationRate: formState.value.depreciationRate,
        },
        [
          'serial',
          'modelName',
          'modelNumber',
          'categoryId',
          'supplierId',
          'locationId',
          'status',
          'warrantyMonths',
          'purchaseDate',
          'orderNumber',
          'purchaseCost',
          'notes',
          'salvageValue',
          'usefulLifeYears',
          'depreciationRate',
        ],
      );
      notification.success({
        message: $t('asset.page.asset.updateSuccess'),
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
    serial: '',
    modelName: '',
    modelNumber: '',
    categoryId: undefined,
    supplierId: undefined,
    locationId: undefined,
    status: 'ASSET_STATUS_DEPLOYABLE',
    warrantyMonths: undefined,
    purchaseDate: '',
    orderNumber: '',
    purchaseCost: undefined,
    notes: '',
    salvageValue: undefined,
    usefulLifeYears: undefined,
    depreciationRate: 0.40,
  };
  assignmentHistory.value = [];
  photoUrl.value = undefined;
  documents.value = [];
}

const [Modal, modalApi] = useVbenModal({
  onCancel() {
    modalApi.close();
  },

  async onOpenChange(isOpen) {
    if (isOpen) {
      data.value = modalApi.getData() as {
        mode: 'create' | 'edit' | 'view';
        row?: Asset;
      };

      activeTab.value = 'general';
      await loadOptions();

      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        formState.value = {
          serial: data.value.row.serial ?? '',
          modelName: data.value.row.modelName ?? '',
          modelNumber: data.value.row.modelNumber ?? '',
          categoryId: data.value.row.categoryId || undefined,
          supplierId: data.value.row.supplierId || undefined,
          locationId: data.value.row.locationId || undefined,
          status: data.value.row.status ?? 'ASSET_STATUS_DEPLOYABLE',
          warrantyMonths: data.value.row.warrantyMonths,
          purchaseDate: data.value.row.purchaseDate ?? '',
          orderNumber: data.value.row.orderNumber ?? '',
          purchaseCost: data.value.row.purchaseCost,
          notes: data.value.row.notes ?? '',
          salvageValue: data.value.row.salvageValue,
          usefulLifeYears: data.value.row.usefulLifeYears,
          depreciationRate: data.value.row.depreciationRate ?? 0.40,
        };

        if (data.value.row.id) {
          await loadAssignmentHistory(data.value.row.id);
          await loadDocuments(data.value.row.id);
          photoUrl.value = data.value.row.photoKey
            ? `/admin/v1/modules/asset/v1/assets/${data.value.row.id}/photo?t=${Date.now()}`
            : undefined;
        }
      }
    } else {
      assignmentHistory.value = [];
      photoUrl.value = undefined;
      documents.value = [];
    }
  },
});

const asset = computed(() => data.value?.row);

const assignmentColumns = [
  {
    title: $t('asset.page.asset.action'),
    dataIndex: 'action',
    key: 'action',
    width: 120,
    customRender: ({ text }: { text: string }) => actionToName(text),
  },
  {
    title: $t('asset.page.asset.employeeId'),
    dataIndex: 'employeeId',
    key: 'employeeId',
    width: 150,
    customRender: ({ text }: { text: string }) =>
      getOptionLabel(employeeOptions.value, text),
  },
  {
    title: $t('asset.page.asset.assignedAt'),
    dataIndex: 'assignedAt',
    key: 'assignedAt',
    width: 160,
  },
  {
    title: $t('asset.page.asset.returnedAt'),
    dataIndex: 'returnedAt',
    key: 'returnedAt',
    width: 160,
  },
  {
    title: $t('asset.page.asset.notes'),
    dataIndex: 'notes',
    key: 'notes',
  },
];

const documentColumns = [
  {
    title: $t('asset.page.asset.fileName'),
    dataIndex: 'fileName',
    key: 'fileName',
    ellipsis: true,
  },
  {
    title: $t('asset.page.asset.fileSize'),
    dataIndex: 'fileSize',
    key: 'fileSize',
    width: 100,
    customRender: ({ text }: { text: number }) => formatFileSize(text),
  },
  {
    title: $t('asset.page.asset.description'),
    dataIndex: 'description',
    key: 'description',
    width: 150,
    ellipsis: true,
  },
  {
    title: $t('asset.page.asset.action'),
    key: 'actions',
    width: 100,
    fixed: 'right' as const,
  },
];
</script>

<template>
  <Modal :title="title" :footer="false" class="w-[800px]">
    <!-- View Mode -->
    <template v-if="asset && isViewMode">
      <Tabs v-model:activeKey="activeTab">
        <TabPane key="general" :tab="$t('asset.page.asset.tabGeneral')">
          <Descriptions :column="1" bordered size="small">
            <DescriptionsItem :label="$t('asset.page.asset.assetTag')">
              <Tag color="blue">{{ asset.assetTag || '-' }}</Tag>
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.status')">
              <Tag :color="statusToColor(asset.status)">
                {{ statusToName(asset.status) }}
              </Tag>
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.serial')">
              {{ asset.serial || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.modelName')">
              {{ asset.modelName || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.modelNumber')">
              {{ asset.modelNumber || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.categoryId')">
              {{ getOptionLabel(categoryOptions, asset.categoryId) }}
            </DescriptionsItem>
            <DescriptionsItem
              v-if="asset.notes"
              :label="$t('asset.page.asset.notes')"
            >
              {{ asset.notes }}
            </DescriptionsItem>
          </Descriptions>
        </TabPane>

        <TabPane
          key="procurement"
          :tab="$t('asset.page.asset.tabProcurement')"
        >
          <Descriptions :column="1" bordered size="small">
            <DescriptionsItem :label="$t('asset.page.asset.supplierId')">
              {{ getOptionLabel(supplierOptions, asset.supplierId) }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.purchaseDate')">
              {{ asset.purchaseDate || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.purchaseCost')">
              {{
                asset.purchaseCost !== undefined ? asset.purchaseCost : '-'
              }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.orderNumber')">
              {{ asset.orderNumber || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.warrantyMonths')">
              {{ asset.warrantyMonths ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.salvageValue')">
              {{ asset.salvageValue ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.usefulLifeYears')">
              {{ asset.usefulLifeYears ?? '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.depreciationRate')">
              {{ asset.depreciationRate ?? '-' }}
            </DescriptionsItem>
          </Descriptions>
        </TabPane>

        <TabPane
          key="assignment"
          :tab="$t('asset.page.asset.tabAssignment')"
        >
          <Descriptions :column="1" bordered size="small" class="mb-4">
            <DescriptionsItem :label="$t('asset.page.asset.locationId')">
              {{ getOptionLabel(locationOptions, asset.locationId) }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.asset.employeeId')">
              {{ getOptionLabel(employeeOptions, asset.employeeId) }}
            </DescriptionsItem>
          </Descriptions>

          <template v-if="assignmentHistory.length > 0">
            <h4 class="mb-2 font-medium">
              {{ $t('asset.page.asset.assignmentHistory') }}
            </h4>
            <Table
              :columns="assignmentColumns"
              :data-source="assignmentHistory"
              :pagination="false"
              size="small"
              bordered
              :row-key="(r: any) => r.id"
            />
          </template>
        </TabPane>

        <TabPane
          key="documents"
          :tab="$t('asset.page.asset.tabDocuments')"
        >
          <!-- Photo -->
          <h4 class="mb-2 font-medium">
            {{ $t('asset.page.asset.sectionPhoto') }}
          </h4>
          <Spin :spinning="photoLoading">
            <div v-if="photoUrl" class="flex flex-col items-center gap-2">
              <Image :src="photoUrl" :width="200" />
            </div>
            <div
              v-else
              class="flex items-center justify-center p-4 text-gray-400"
            >
              <LucideFile class="mr-2 size-5" />
              {{ $t('asset.page.asset.noPhoto') }}
            </div>
          </Spin>

          <!-- Documents -->
          <h4 class="mb-2 mt-4 font-medium">
            {{ $t('asset.page.asset.sectionDocuments') }}
          </h4>
          <Spin :spinning="documentsLoading">
            <Table
              v-if="documents.length > 0"
              :columns="documentColumns"
              :data-source="documents"
              :pagination="false"
              size="small"
              bordered
              :row-key="(r: any) => r.id"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'actions'">
                  <Button
                    type="link"
                    size="small"
                    :title="$t('ui.button.download')"
                    @click="
                      handleDownloadDocument(
                        record as unknown as AssetDocument,
                      )
                    "
                  >
                    <template #icon>
                      <LucideDownload class="size-4" />
                    </template>
                  </Button>
                </template>
              </template>
            </Table>
            <div
              v-else
              class="flex items-center justify-center p-4 text-gray-400"
            >
              <LucideFile class="mr-2 size-5" />
              {{ $t('asset.page.asset.noDocuments') }}
            </div>
          </Spin>
        </TabPane>
      </Tabs>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <Tabs v-model:activeKey="activeTab">
          <TabPane key="general" :tab="$t('asset.page.asset.tabGeneral')">
            <!-- Show asset tag as read-only badge in edit mode -->
            <div
              v-if="isEditMode && data?.row?.assetTag"
              class="mb-4"
            >
              <label class="mb-1 block text-sm font-medium">{{
                $t('asset.page.asset.assetTag')
              }}</label>
              <Tag color="blue">{{ data.row.assetTag }}</Tag>
            </div>

            <FormItem :label="$t('asset.page.asset.serial')" name="serial">
              <Input
                v-model:value="formState.serial"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.asset.modelName')"
              name="modelName"
            >
              <Input
                v-model:value="formState.modelName"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.asset.modelNumber')"
              name="modelNumber"
            >
              <Input
                v-model:value="formState.modelNumber"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.asset.categoryId')"
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

            <FormItem :label="$t('asset.page.asset.status')" name="status">
              <Select
                v-model:value="formState.status"
                :options="statusOptions"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.asset.notes')" name="notes">
              <Textarea
                v-model:value="formState.notes"
                :rows="3"
                :maxlength="1024"
                :placeholder="$t('ui.placeholder.input')"
              />
            </FormItem>
          </TabPane>

          <TabPane
            key="procurement"
            :tab="$t('asset.page.asset.tabProcurement')"
          >
            <FormItem
              :label="$t('asset.page.asset.supplierId')"
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
              :label="$t('asset.page.asset.purchaseDate')"
              name="purchaseDate"
            >
              <DatePicker
                v-model:value="formState.purchaseDate"
                value-format="YYYY-MM-DD"
                style="width: 100%"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.asset.purchaseCost')"
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
              :label="$t('asset.page.asset.orderNumber')"
              name="orderNumber"
            >
              <Input
                v-model:value="formState.orderNumber"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.asset.warrantyMonths')"
              name="warrantyMonths"
            >
              <InputNumber
                v-model:value="formState.warrantyMonths"
                :min="0"
                style="width: 100%"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.asset.salvageValue')"
              name="salvageValue"
            >
              <InputNumber
                v-model:value="formState.salvageValue"
                :min="0"
                :precision="2"
                style="width: 100%"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.asset.usefulLifeYears')"
              name="usefulLifeYears"
            >
              <InputNumber
                v-model:value="formState.usefulLifeYears"
                :min="0"
                style="width: 100%"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.asset.depreciationRate')"
              name="depreciationRate"
            >
              <InputNumber
                v-model:value="formState.depreciationRate"
                :min="0"
                :max="1"
                :step="0.01"
                :precision="2"
                style="width: 100%"
              />
            </FormItem>
          </TabPane>

          <TabPane
            v-if="isEditMode && data?.row?.id"
            key="assignment"
            :tab="$t('asset.page.asset.tabAssignment')"
          >
            <Descriptions :column="1" bordered size="small" class="mb-4">
              <DescriptionsItem :label="$t('asset.page.asset.locationId')">
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
                  style="width: 100%"
                />
              </DescriptionsItem>
            </Descriptions>

            <template v-if="assignmentHistory.length > 0">
              <h4 class="mb-2 font-medium">
                {{ $t('asset.page.asset.assignmentHistory') }}
              </h4>
              <Table
                :columns="assignmentColumns"
                :data-source="assignmentHistory"
                :pagination="false"
                size="small"
                bordered
                :row-key="(r: any) => r.id"
              />
            </template>
          </TabPane>

          <TabPane
            v-if="isEditMode && data?.row?.id"
            key="documents"
            :tab="$t('asset.page.asset.tabDocuments')"
          >
            <!-- Photo (Edit) -->
            <h4 class="mb-2 font-medium">
              {{ $t('asset.page.asset.sectionPhoto') }}
            </h4>
            <Spin :spinning="photoLoading">
              <div v-if="photoUrl" class="flex flex-col items-center gap-3">
                <Image :src="photoUrl" :width="200" />
                <Popconfirm
                  :title="$t('asset.page.asset.deletePhoto') + '?'"
                  :ok-text="$t('ui.button.ok')"
                  :cancel-text="$t('ui.button.cancel')"
                  @confirm="handleDeletePhoto"
                >
                  <Button danger size="small">
                    <template #icon>
                      <LucideTrash class="size-4" />
                    </template>
                    {{ $t('asset.page.asset.deletePhoto') }}
                  </Button>
                </Popconfirm>
              </div>
              <div
                v-else
                class="cursor-pointer rounded-lg border-2 border-dashed p-6 text-center transition-colors"
                :class="
                  isPhotoDragOver
                    ? 'border-blue-400 bg-blue-50/10'
                    : 'border-gray-300 hover:border-gray-400'
                "
                @click.stop="triggerPhotoInput"
                @drop.prevent.stop="handlePhotoDrop"
                @dragover.prevent.stop="isPhotoDragOver = true"
                @dragleave="isPhotoDragOver = false"
              >
                <div
                  class="flex flex-col items-center gap-2 text-gray-400"
                >
                  <LucideUpload class="size-8" />
                  <span>{{ $t('asset.page.asset.dropPhotoHere') }}</span>
                </div>
              </div>
            </Spin>

            <!-- Documents (Edit) -->
            <h4 class="mb-2 mt-4 font-medium">
              {{ $t('asset.page.asset.sectionDocuments') }}
            </h4>
            <Spin :spinning="documentsLoading">
              <Table
                v-if="documents.length > 0"
                :columns="documentColumns"
                :data-source="documents"
                :pagination="false"
                size="small"
                bordered
                :row-key="(r: any) => r.id"
                class="mb-3"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'actions'">
                    <div class="flex items-center gap-1">
                      <Button
                        type="link"
                        size="small"
                        @click="
                          handleDownloadDocument(
                            record as unknown as AssetDocument,
                          )
                        "
                      >
                        <template #icon>
                          <LucideDownload class="size-4" />
                        </template>
                      </Button>
                      <Popconfirm
                        :title="
                          $t('asset.page.asset.confirmDeleteDocument')
                        "
                        :ok-text="$t('ui.button.ok')"
                        :cancel-text="$t('ui.button.cancel')"
                        @confirm="handleDeleteDocument(record.id)"
                      >
                        <Button type="link" size="small" danger>
                          <template #icon>
                            <LucideTrash class="size-4" />
                          </template>
                        </Button>
                      </Popconfirm>
                    </div>
                  </template>
                </template>
              </Table>
              <div
                class="cursor-pointer rounded-lg border-2 border-dashed p-6 text-center transition-colors"
                :class="
                  isDocDragOver
                    ? 'border-blue-400 bg-blue-50/10'
                    : 'border-gray-300 hover:border-gray-400'
                "
                @click.stop="triggerDocInput"
                @drop.prevent.stop="handleDocDrop"
                @dragover.prevent.stop="isDocDragOver = true"
                @dragleave="isDocDragOver = false"
              >
                <div
                  class="flex flex-col items-center gap-2 text-gray-400"
                >
                  <LucideUpload class="size-8" />
                  <span>{{ $t('asset.page.asset.dropFileHere') }}</span>
                </div>
              </div>
            </Spin>
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

    <!-- Hidden file inputs -->
    <Teleport to="body">
      <input
        ref="photoInputRef"
        type="file"
        accept="image/*"
        style="position: fixed; top: -9999px; left: -9999px; opacity: 0"
        @change="handlePhotoInputChange"
      />
      <input
        ref="docInputRef"
        type="file"
        style="position: fixed; top: -9999px; left: -9999px; opacity: 0"
        @change="handleDocInputChange"
      />
    </Teleport>
  </Modal>
</template>
