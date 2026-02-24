import { defineStore } from 'pinia';

import {
  EmployeeService,
  type Employee,
  type LdapSyncExecuteResponse,
  type LdapSyncPreviewResponse,
  type ListEmployeesResponse,
} from '../api/services';
import type { Paging } from '../api/services';

export const useAssetEmployeeStore = defineStore('asset-employee', () => {
  async function listEmployees(
    paging?: Paging,
    formValues?: {
      query?: string;
      department?: string;
    } | null,
  ): Promise<ListEmployeesResponse> {
    return await EmployeeService.list({
      query: formValues?.query,
      department: formValues?.department,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  async function getEmployee(id: string): Promise<{ employee: Employee }> {
    return await EmployeeService.get(id);
  }

  async function createEmployee(
    data: Partial<Employee>,
  ): Promise<{ employee: Employee }> {
    return await EmployeeService.create(data);
  }

  async function updateEmployee(
    id: string,
    data: Partial<Employee>,
    updateMask: string[],
  ): Promise<{ employee: Employee }> {
    return await EmployeeService.update(id, {
      id,
      data: data as Employee,
      updateMask: updateMask.join(','),
    });
  }

  async function deleteEmployee(id: string): Promise<void> {
    return await EmployeeService.delete(id);
  }

  async function ldapSyncPreview(): Promise<LdapSyncPreviewResponse> {
    return await EmployeeService.ldapSyncPreview();
  }

  async function ldapSyncExecute(
    selectedDns?: string[],
  ): Promise<LdapSyncExecuteResponse> {
    return await EmployeeService.ldapSyncExecute(
      selectedDns ? { selectedDns } : undefined,
    );
  }

  function $reset() {}

  return {
    $reset,
    listEmployees,
    getEmployee,
    createEmployee,
    updateEmployee,
    deleteEmployee,
    ldapSyncPreview,
    ldapSyncExecute,
  };
});
