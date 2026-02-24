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
  Descriptions,
  DescriptionsItem,
  Tabs,
  TabPane,
} from 'ant-design-vue';

import type { Employee } from '../../api/services';
import { $t } from 'shell/locales';
import { useAssetEmployeeStore } from '../../stores/asset-employee.state';

const employeeStore = useAssetEmployeeStore();

const data = ref<{
  mode: 'create' | 'edit' | 'view';
  row?: Employee;
}>();
const loading = ref(false);
const activeTab = ref('personal');

const formState = ref({
  firstName: '',
  lastName: '',
  email: '',
  phone: '',
  department: '',
  jobTitle: '',
  employeeNumber: '',
  notes: '',
});

const title = computed(() => {
  switch (data.value?.mode) {
    case 'create':
      return $t('asset.page.employee.create');
    case 'edit':
      return $t('asset.page.employee.edit');
    default:
      return $t('asset.page.employee.view');
  }
});

const isCreateMode = computed(() => data.value?.mode === 'create');
const isEditMode = computed(() => data.value?.mode === 'edit');
const isViewMode = computed(() => data.value?.mode === 'view');

async function handleSubmit() {
  loading.value = true;
  try {
    if (isCreateMode.value) {
      await employeeStore.createEmployee({
        firstName: formState.value.firstName,
        lastName: formState.value.lastName,
        email: formState.value.email || undefined,
        phone: formState.value.phone || undefined,
        department: formState.value.department || undefined,
        jobTitle: formState.value.jobTitle || undefined,
        employeeNumber: formState.value.employeeNumber || undefined,
        notes: formState.value.notes || undefined,
      });
      notification.success({
        message: $t('asset.page.employee.createSuccess'),
      });
    } else if (isEditMode.value && data.value?.row?.id) {
      await employeeStore.updateEmployee(
        data.value.row.id,
        {
          firstName: formState.value.firstName,
          lastName: formState.value.lastName,
          email: formState.value.email || undefined,
          phone: formState.value.phone || undefined,
          department: formState.value.department || undefined,
          jobTitle: formState.value.jobTitle || undefined,
          employeeNumber: formState.value.employeeNumber || undefined,
          notes: formState.value.notes || undefined,
        },
        [
          'firstName',
          'lastName',
          'email',
          'phone',
          'department',
          'jobTitle',
          'employeeNumber',
          'notes',
        ],
      );
      notification.success({
        message: $t('asset.page.employee.updateSuccess'),
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
    firstName: '',
    lastName: '',
    email: '',
    phone: '',
    department: '',
    jobTitle: '',
    employeeNumber: '',
    notes: '',
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
        row?: Employee;
      };

      activeTab.value = 'personal';

      if (data.value?.mode === 'create') {
        resetForm();
      } else if (data.value?.row) {
        formState.value = {
          firstName: data.value.row.firstName ?? '',
          lastName: data.value.row.lastName ?? '',
          email: data.value.row.email ?? '',
          phone: data.value.row.phone ?? '',
          department: data.value.row.department ?? '',
          jobTitle: data.value.row.jobTitle ?? '',
          employeeNumber: data.value.row.employeeNumber ?? '',
          notes: data.value.row.notes ?? '',
        };
      }
    }
  },
});

const employee = computed(() => data.value?.row);
</script>

<template>
  <Modal :title="title" :footer="false" class="w-[700px]">
    <!-- View Mode -->
    <template v-if="employee && isViewMode">
      <Tabs v-model:activeKey="activeTab">
        <TabPane key="personal" :tab="$t('asset.page.employee.tabPersonal')">
          <Descriptions :column="1" bordered size="small">
            <DescriptionsItem :label="$t('asset.page.employee.firstName')">
              {{ employee.firstName }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.employee.lastName')">
              {{ employee.lastName }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.employee.email')">
              {{ employee.email || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.employee.phone')">
              {{ employee.phone || '-' }}
            </DescriptionsItem>
            <DescriptionsItem
              v-if="employee.notes"
              :label="$t('asset.page.employee.notes')"
            >
              {{ employee.notes }}
            </DescriptionsItem>
          </Descriptions>
        </TabPane>

        <TabPane key="work" :tab="$t('asset.page.employee.tabWork')">
          <Descriptions :column="1" bordered size="small">
            <DescriptionsItem
              :label="$t('asset.page.employee.employeeNumber')"
            >
              {{ employee.employeeNumber || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.employee.department')">
              {{ employee.department || '-' }}
            </DescriptionsItem>
            <DescriptionsItem :label="$t('asset.page.employee.jobTitle')">
              {{ employee.jobTitle || '-' }}
            </DescriptionsItem>
          </Descriptions>
        </TabPane>
      </Tabs>
    </template>

    <!-- Create/Edit Mode -->
    <template v-else-if="isCreateMode || isEditMode">
      <Form layout="vertical" :model="formState" @finish="handleSubmit">
        <Tabs v-model:activeKey="activeTab">
          <TabPane
            key="personal"
            :tab="$t('asset.page.employee.tabPersonal')"
          >
            <FormItem
              :label="$t('asset.page.employee.firstName')"
              name="firstName"
              :rules="[
                { required: true, message: $t('ui.formRules.required') },
              ]"
            >
              <Input
                v-model:value="formState.firstName"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.employee.lastName')"
              name="lastName"
              :rules="[
                { required: true, message: $t('ui.formRules.required') },
              ]"
            >
              <Input
                v-model:value="formState.lastName"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.employee.email')" name="email">
              <Input
                v-model:value="formState.email"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.employee.phone')" name="phone">
              <Input
                v-model:value="formState.phone"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="50"
              />
            </FormItem>

            <FormItem :label="$t('asset.page.employee.notes')" name="notes">
              <Textarea
                v-model:value="formState.notes"
                :rows="3"
                :maxlength="1024"
                :placeholder="$t('ui.placeholder.input')"
              />
            </FormItem>
          </TabPane>

          <TabPane key="work" :tab="$t('asset.page.employee.tabWork')">
            <FormItem
              :label="$t('asset.page.employee.employeeNumber')"
              name="employeeNumber"
            >
              <Input
                v-model:value="formState.employeeNumber"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="100"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.employee.department')"
              name="department"
            >
              <Input
                v-model:value="formState.department"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
              />
            </FormItem>

            <FormItem
              :label="$t('asset.page.employee.jobTitle')"
              name="jobTitle"
            >
              <Input
                v-model:value="formState.jobTitle"
                :placeholder="$t('ui.placeholder.input')"
                :maxlength="255"
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
