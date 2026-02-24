<script lang="ts" setup>
import { ref, computed } from 'vue';

import { useVbenModal } from 'shell/vben/common-ui';

import {
  Alert,
  Button,
  notification,
  Table,
  Tag,
  Spin,
} from 'ant-design-vue';

import type {
  LdapSyncChange,
  LdapSyncPreviewResponse,
  LdapSyncExecuteResponse,
} from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetEmployeeStore } from '../../stores/asset-employee.state';

const employeeStore = useAssetEmployeeStore();

const loading = ref(false);
const step = ref<'preview' | 'result'>('preview');
const preview = ref<LdapSyncPreviewResponse | null>(null);
const result = ref<LdapSyncExecuteResponse | null>(null);
const error = ref<string | null>(null);

const hasChanges = computed(() => {
  return preview.value && (preview.value.newCount > 0 || preview.value.updateCount > 0);
});

const columns = [
  {
    title: $t('asset.page.employee.ldap.action'),
    dataIndex: 'action',
    key: 'action',
    width: 100,
  },
  {
    title: $t('asset.page.employee.fullName'),
    key: 'name',
    width: 180,
  },
  {
    title: $t('asset.page.employee.email'),
    key: 'email',
    width: 200,
  },
  {
    title: $t('asset.page.employee.department'),
    key: 'department',
    width: 140,
  },
  {
    title: $t('asset.page.employee.ldap.changedFields'),
    key: 'changedFields',
  },
];

function actionColor(action: string) {
  return action === 'ACTION_CREATE' ? 'green' : 'blue';
}

function actionLabel(action: string) {
  return action === 'ACTION_CREATE'
    ? $t('asset.page.employee.ldap.newEmployees')
    : $t('asset.page.employee.ldap.updatedEmployees');
}

function fieldLabel(field: string) {
  const map: Record<string, string> = {
    first_name: $t('asset.page.employee.firstName'),
    last_name: $t('asset.page.employee.lastName'),
    email: $t('asset.page.employee.email'),
    phone: $t('asset.page.employee.phone'),
    department: $t('asset.page.employee.department'),
    job_title: $t('asset.page.employee.jobTitle'),
    employee_number: $t('asset.page.employee.employeeNumber'),
  };
  return map[field] || field;
}

async function loadPreview() {
  loading.value = true;
  error.value = null;
  preview.value = null;

  try {
    preview.value = await employeeStore.ldapSyncPreview();
  } catch (e: any) {
    error.value = e.message || $t('asset.page.employee.ldap.syncFailed');
  } finally {
    loading.value = false;
  }
}

async function handleApply() {
  loading.value = true;
  error.value = null;

  try {
    result.value = await employeeStore.ldapSyncExecute();
    step.value = 'result';

    if (result.value.errorCount > 0) {
      notification.warning({
        message: $t('asset.page.employee.ldap.syncPartial'),
      });
    } else {
      notification.success({
        message: $t('asset.page.employee.ldap.syncSuccess'),
      });
    }
  } catch (e: any) {
    error.value = e.message || $t('asset.page.employee.ldap.syncFailed');
  } finally {
    loading.value = false;
  }
}

function resetState() {
  step.value = 'preview';
  preview.value = null;
  result.value = null;
  error.value = null;
  loading.value = false;
}

const [Modal, modalApi] = useVbenModal({
  onCancel() {
    modalApi.close();
  },

  onOpenChange(isOpen) {
    if (isOpen) {
      resetState();
      loadPreview();
    }
  },
});
</script>

<template>
  <Modal
    :title="$t('asset.page.employee.ldap.title')"
    :footer="false"
    class="w-[900px]"
  >
    <div class="space-y-4">
      <!-- Loading -->
      <div v-if="loading && step === 'preview'" class="py-12 text-center">
        <Spin size="large" />
        <p class="mt-4 text-gray-500">
          {{ step === 'preview' ? $t('asset.page.employee.ldap.fetching') : $t('asset.page.employee.ldap.syncing') }}
        </p>
      </div>

      <!-- Error -->
      <Alert
        v-if="error"
        type="error"
        :message="$t('asset.page.employee.ldap.syncFailed')"
        :description="error"
        show-icon
        class="mb-4"
      />

      <!-- Preview Step -->
      <template v-if="step === 'preview' && !loading && preview">
        <!-- Summary -->
        <div class="flex items-center gap-4 mb-4">
          <span class="text-sm text-gray-500">
            {{ $t('asset.page.employee.ldap.totalLdap') }}:
            <strong>{{ preview.totalLdapEntries }}</strong>
          </span>
          <Tag color="green">
            {{ $t('asset.page.employee.ldap.newEmployees') }}: {{ preview.newCount }}
          </Tag>
          <Tag color="blue">
            {{ $t('asset.page.employee.ldap.updatedEmployees') }}: {{ preview.updateCount }}
          </Tag>
          <Tag>
            {{ $t('asset.page.employee.ldap.unchanged') }}: {{ preview.unchangedCount }}
          </Tag>
        </div>

        <!-- Warnings -->
        <Alert
          v-if="preview.warnings && preview.warnings.length > 0"
          type="warning"
          :message="$t('asset.page.employee.ldap.warnings')"
          show-icon
          class="mb-4"
        >
          <template #description>
            <ul class="list-disc list-inside mt-1">
              <li v-for="w in preview.warnings" :key="w">{{ w }}</li>
            </ul>
          </template>
        </Alert>

        <!-- No Changes -->
        <Alert
          v-if="!hasChanges"
          type="info"
          :message="$t('asset.page.employee.ldap.noChanges')"
          show-icon
        />

        <!-- Changes Table -->
        <Table
          v-if="hasChanges"
          :data-source="preview.changes"
          :columns="columns"
          :pagination="false"
          :scroll="{ y: 400 }"
          size="small"
          row-key="ldapDn"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'action'">
              <Tag :color="actionColor((record as LdapSyncChange).action)">
                {{ actionLabel((record as LdapSyncChange).action) }}
              </Tag>
            </template>
            <template v-else-if="column.key === 'name'">
              {{ (record as LdapSyncChange).employee?.firstName }}
              {{ (record as LdapSyncChange).employee?.lastName }}
            </template>
            <template v-else-if="column.key === 'email'">
              {{ (record as LdapSyncChange).employee?.email || '-' }}
            </template>
            <template v-else-if="column.key === 'department'">
              {{ (record as LdapSyncChange).employee?.department || '-' }}
            </template>
            <template v-else-if="column.key === 'changedFields'">
              <template v-if="(record as LdapSyncChange).changedFields?.length">
                <Tag
                  v-for="f in (record as LdapSyncChange).changedFields"
                  :key="f"
                  size="small"
                  class="mr-1"
                >
                  {{ fieldLabel(f) }}
                </Tag>
              </template>
              <span v-else class="text-gray-400">-</span>
            </template>
          </template>
        </Table>

        <!-- Apply Button -->
        <div v-if="hasChanges" class="mt-4 flex justify-end">
          <Button type="primary" :loading="loading" @click="handleApply">
            {{ $t('asset.page.employee.ldap.sync') }}
          </Button>
        </div>
      </template>

      <!-- Result Step -->
      <template v-if="step === 'result' && result">
        <Alert
          :type="result.errorCount > 0 ? 'warning' : 'success'"
          :message="$t('asset.page.employee.ldap.syncComplete')"
          show-icon
          class="mb-4"
        />

        <div class="flex items-center gap-4 mb-4">
          <Tag color="green">
            {{ $t('asset.page.employee.ldap.created') }}: {{ result.createdCount }}
          </Tag>
          <Tag color="blue">
            {{ $t('asset.page.employee.ldap.updated') }}: {{ result.updatedCount }}
          </Tag>
          <Tag v-if="result.skippedCount > 0">
            {{ $t('asset.page.employee.ldap.skipped') }}: {{ result.skippedCount }}
          </Tag>
          <Tag v-if="result.errorCount > 0" color="red">
            {{ $t('asset.page.employee.ldap.errors') }}: {{ result.errorCount }}
          </Tag>
        </div>

        <Alert
          v-if="result.errors && result.errors.length > 0"
          type="error"
          :message="$t('asset.page.employee.ldap.errors')"
          show-icon
        >
          <template #description>
            <ul class="list-disc list-inside mt-1">
              <li v-for="e in result.errors" :key="e">{{ e }}</li>
            </ul>
          </template>
        </Alert>

        <div class="mt-4 flex justify-end">
          <Button @click="modalApi.close()">
            {{ $t('ui.button.close') }}
          </Button>
        </div>
      </template>
    </div>
  </Modal>
</template>
