/**
 * Asset Module Service Functions
 *
 * Typed service methods for the Asset API using dynamic module routing.
 * Base URL: /admin/v1/modules/asset/v1
 */

import { assetApi, type RequestOptions } from './client';

// Paging utility type (replaces #/utils/request import in stores)
export interface Paging {
  page?: number;
  pageSize?: number;
}

// ==================== Enum Types ====================

export type AssetStatus =
  | 'ASSET_STATUS_ARCHIVED'
  | 'ASSET_STATUS_ASSIGNED'
  | 'ASSET_STATUS_BROKEN'
  | 'ASSET_STATUS_DEPLOYABLE'
  | 'ASSET_STATUS_UNSPECIFIED';

export type LocationStatus =
  | 'LOCATION_STATUS_ACTIVE'
  | 'LOCATION_STATUS_DECOMMISSIONED'
  | 'LOCATION_STATUS_PLANNED'
  | 'LOCATION_STATUS_UNSPECIFIED';

export type AssignmentAction =
  | 'ASSIGNMENT_ACTION_ASSIGNED'
  | 'ASSIGNMENT_ACTION_TRANSFERRED'
  | 'ASSIGNMENT_ACTION_UNASSIGNED'
  | 'ASSIGNMENT_ACTION_UNSPECIFIED';

// ==================== Entity Types ====================

export interface Supplier {
  id: string;
  tenantId?: number;
  name: string;
  code?: string;
  address?: string;
  city?: string;
  state?: string;
  country?: string;
  postalCode?: string;
  contactPerson?: string;
  telephone?: string;
  email?: string;
  website?: string;
  notes?: string;
  tags?: string;
  metadata?: Record<string, unknown>;
  status?: string;
  createTime?: string;
  updateTime?: string;
  createdBy?: number;
  updatedBy?: number;
}

export interface PortalUser {
  id: number;
  username: string;
  realname: string;
  email?: string;
  orgUnitNames?: string[];
  positionNames?: string[];
}

export interface Location {
  id: string;
  tenantId?: number;
  name: string;
  code?: string;
  description?: string;
  parentId?: string;
  path?: string;
  address?: string;
  city?: string;
  state?: string;
  country?: string;
  postalCode?: string;
  contact?: string;
  phone?: string;
  email?: string;
  status?: LocationStatus;
  childCount?: number;
  assetCount?: number;
  tags?: string;
  metadata?: Record<string, unknown>;
  createTime?: string;
  updateTime?: string;
  createdBy?: number;
  updatedBy?: number;
}

export interface LocationTreeNode {
  location?: Location;
  children?: LocationTreeNode[];
}

export interface Category {
  id: string;
  tenantId?: number;
  name: string;
  description?: string;
  parentId?: string;
  icon?: string;
  metadata?: Record<string, unknown>;
  assetCount?: number;
  childCount?: number;
  createTime?: string;
  updateTime?: string;
  createdBy?: number;
  updatedBy?: number;
}

export interface CategoryTreeNode {
  category?: Category;
  children?: CategoryTreeNode[];
}

export interface Asset {
  id: string;
  tenantId?: number;
  assetTag?: string;
  name?: string;
  serial?: string;
  modelName?: string;
  modelNumber?: string;
  categoryId?: string;
  supplierId?: string;
  locationId?: string;
  userId?: number;
  status?: AssetStatus;
  photoKey?: string;
  warrantyMonths?: number;
  purchaseDate?: string;
  orderNumber?: string;
  purchaseCost?: number;
  notes?: string;
  tags?: string;
  metadata?: Record<string, unknown>;
  salvageValue?: number;
  usefulLifeYears?: number;
  depreciationRate?: number;
  createTime?: string;
  updateTime?: string;
  createdBy?: number;
  updatedBy?: number;
}

export interface AssetAssignment {
  id: string;
  assetId: string;
  assetName?: string;
  userId?: number;
  userName?: string;
  action?: AssignmentAction;
  assignedAt?: string;
  returnedAt?: string;
  assignedBy?: number;
  notes?: string;
}

export interface AssetDocument {
  id: string;
  assetId?: string;
  consumableId?: string;
  fileName: string;
  fileSize?: number;
  mimeType?: string;
  storageKey?: string;
  checksum?: string;
  description?: string;
  createTime?: string;
  createdBy?: number;
}

export interface Consumable {
  id: string;
  tenantId?: number;
  name: string;
  description?: string;
  categoryId?: string;
  supplierId?: string;
  locationId?: string;
  modelName?: string;
  modelNumber?: string;
  amount?: number;
  minAmount?: number;
  purchaseDate?: string;
  purchaseCost?: number;
  orderNumber?: string;
  notes?: string;
  tags?: string;
  metadata?: Record<string, unknown>;
  createTime?: string;
  updateTime?: string;
  createdBy?: number;
  updatedBy?: number;
}

export type LicenseStatus =
  | 'LICENSE_STATUS_ACTIVE'
  | 'LICENSE_STATUS_EXPIRED'
  | 'LICENSE_STATUS_SUSPENDED'
  | 'LICENSE_STATUS_UNSPECIFIED';

export type InsurancePolicyStatus =
  | 'INSURANCE_POLICY_STATUS_ACTIVE'
  | 'INSURANCE_POLICY_STATUS_CANCELLED'
  | 'INSURANCE_POLICY_STATUS_EXPIRED'
  | 'INSURANCE_POLICY_STATUS_UNSPECIFIED';

export type CoverageType =
  | 'COVERAGE_TYPE_ALL_RISK'
  | 'COVERAGE_TYPE_CYBER'
  | 'COVERAGE_TYPE_EQUIPMENT_BREAKDOWN'
  | 'COVERAGE_TYPE_FIRE_THEFT'
  | 'COVERAGE_TYPE_LIABILITY'
  | 'COVERAGE_TYPE_UNSPECIFIED';

export interface License {
  id: string;
  tenantId?: number;
  name: string;
  supplierId?: string;
  purchaseDate?: string;
  purchaseCost?: number;
  orderNumber?: string;
  validFrom?: string;
  validTo?: string;
  notes?: string;
  metadata?: Record<string, unknown>;
  status?: LicenseStatus;
  createdAt?: string;
  updatedAt?: string;
  createdBy?: number;
  updatedBy?: number;
}

export interface ListLicensesResponse {
  items: License[];
  total: number;
}

export interface InsurancePolicy {
  id: string;
  tenantId?: number;
  name: string;
  policyNumber?: string;
  provider?: string;
  coverageType?: CoverageType;
  premiumAmount?: number;
  deductible?: number;
  coverageLimit?: number;
  validFrom?: string;
  validTo?: string;
  status?: InsurancePolicyStatus;
  notes?: string;
  metadata?: Record<string, unknown>;
  assetCount?: number;
  createdAt?: string;
  updatedAt?: string;
  createdBy?: number;
  updatedBy?: number;
}

export interface PolicyAsset {
  policyId: string;
  assetId: string;
  coveredValue?: number;
  notes?: string;
  assetTag?: string;
  assetName?: string;
  modelName?: string;
}

export interface ListInsurancePoliciesResponse {
  items: InsurancePolicy[];
  total: number;
}

export interface ListPolicyAssetsResponse {
  items: PolicyAsset[];
}

export interface HealthCheckResponse {
  status: string;
  version?: string;
  timestamp?: string;
}

export interface DashboardStats {
  totalAssets?: number;
  deployableAssets?: number;
  assignedAssets?: number;
  brokenAssets?: number;
  archivedAssets?: number;
  totalConsumables?: number;
  totalSuppliers?: number;
  totalCategories?: number;
  totalLocations?: number;
  totalCost?: number;
}

// ==================== Response Types ====================

export interface ListSuppliersResponse {
  items: Supplier[];
  total: number;
}

export interface ListUsersResponse {
  items: PortalUser[];
  total: number;
}

export interface ListLocationsResponse {
  items: Location[];
  total: number;
}

export interface GetLocationTreeResponse {
  nodes: LocationTreeNode[];
}

export interface ListCategoriesResponse {
  items: Category[];
  total: number;
}

export interface GetCategoryTreeResponse {
  nodes: CategoryTreeNode[];
}

export interface ListAssetsResponse {
  items: Asset[];
  total: number;
}

export interface ListConsumablesResponse {
  items: Consumable[];
  total: number;
}

export interface ListAssignmentsResponse {
  items: AssetAssignment[];
  total: number;
}

export interface ListDocumentsResponse {
  items: AssetDocument[];
  total: number;
}

// ==================== Inventory Sync Types ====================

export type InventorySyncAction =
  | 'ACTION_CREATE'
  | 'ACTION_UNSPECIFIED'
  | 'ACTION_UPDATE';

export interface InventorySyncChange {
  action: InventorySyncAction;
  hostname?: string;
  serial?: string;
  modelName?: string;
  changedFields?: string[];
  existingId?: string;
}

export interface InventorySyncPreviewResponse {
  totalInventoryEntries: number;
  newCount: number;
  updateCount: number;
  unchangedCount: number;
  changes: InventorySyncChange[];
  warnings?: string[];
}

export interface InventorySyncExecuteResponse {
  createdCount: number;
  updatedCount: number;
  skippedCount: number;
  errorCount: number;
  errors?: string[];
}

// ==================== Helper ====================

function buildQuery(params: Record<string, unknown>): string {
  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      if (Array.isArray(value)) {
        value.forEach((v) => searchParams.append(key, String(v)));
      } else {
        searchParams.append(key, String(value));
      }
    }
  }
  const query = searchParams.toString();
  return query ? `?${query}` : '';
}

// ==================== Supplier Service ====================

export const SupplierService = {
  list: async (
    params?: {
      query?: string;
      status?: string;
      page?: number;
      pageSize?: number;
      noPaging?: boolean;
    },
    options?: RequestOptions,
  ): Promise<ListSuppliersResponse> => {
    return assetApi.get<ListSuppliersResponse>(
      `/suppliers${buildQuery(params || {})}`,
      options,
    );
  },

  get: async (
    id: string,
    options?: RequestOptions,
  ): Promise<{ supplier: Supplier }> => {
    return assetApi.get<{ supplier: Supplier }>(`/suppliers/${id}`, options);
  },

  create: async (
    data: Partial<Supplier>,
    options?: RequestOptions,
  ): Promise<{ supplier: Supplier }> => {
    return assetApi.post<{ supplier: Supplier }>('/suppliers', data, options);
  },

  update: async (
    id: string,
    data: { id: string; data: Supplier; updateMask: string },
    options?: RequestOptions,
  ): Promise<{ supplier: Supplier }> => {
    return assetApi.put<{ supplier: Supplier }>(
      `/suppliers/${id}`,
      data,
      options,
    );
  },

  delete: async (id: string, options?: RequestOptions): Promise<void> => {
    return assetApi.delete(`/suppliers/${id}`, options);
  },
};

// ==================== User Service ====================

export const UserService = {
  list: async (
    options?: RequestOptions,
  ): Promise<ListUsersResponse> => {
    return assetApi.get<ListUsersResponse>('/users', options);
  },
};

// ==================== Location Service ====================

export const LocationService = {
  list: async (
    params?: {
      query?: string;
      parentId?: string;
      status?: LocationStatus;
      page?: number;
      pageSize?: number;
      noPaging?: boolean;
    },
    options?: RequestOptions,
  ): Promise<ListLocationsResponse> => {
    return assetApi.get<ListLocationsResponse>(
      `/locations${buildQuery(params || {})}`,
      options,
    );
  },

  get: async (
    id: string,
    options?: RequestOptions,
  ): Promise<{ location: Location }> => {
    return assetApi.get<{ location: Location }>(`/locations/${id}`, options);
  },

  create: async (
    data: Partial<Location>,
    options?: RequestOptions,
  ): Promise<{ location: Location }> => {
    return assetApi.post<{ location: Location }>(
      '/locations',
      data,
      options,
    );
  },

  update: async (
    id: string,
    data: { id: string; data: Location; updateMask: string },
    options?: RequestOptions,
  ): Promise<{ location: Location }> => {
    return assetApi.put<{ location: Location }>(
      `/locations/${id}`,
      data,
      options,
    );
  },

  delete: async (id: string, options?: RequestOptions): Promise<void> => {
    return assetApi.delete(`/locations/${id}`, options);
  },

  getTree: async (
    params?: { rootId?: string; maxDepth?: number },
    options?: RequestOptions,
  ): Promise<GetLocationTreeResponse> => {
    return assetApi.get<GetLocationTreeResponse>(
      `/locations/tree${buildQuery(params || {})}`,
      options,
    );
  },
};

// ==================== Category Service ====================

export const CategoryService = {
  list: async (
    params?: {
      query?: string;
      parentId?: string;
      page?: number;
      pageSize?: number;
      noPaging?: boolean;
    },
    options?: RequestOptions,
  ): Promise<ListCategoriesResponse> => {
    return assetApi.get<ListCategoriesResponse>(
      `/categories${buildQuery(params || {})}`,
      options,
    );
  },

  get: async (
    id: string,
    options?: RequestOptions,
  ): Promise<{ category: Category }> => {
    return assetApi.get<{ category: Category }>(
      `/categories/${id}`,
      options,
    );
  },

  create: async (
    data: Partial<Category>,
    options?: RequestOptions,
  ): Promise<{ category: Category }> => {
    return assetApi.post<{ category: Category }>(
      '/categories',
      data,
      options,
    );
  },

  update: async (
    id: string,
    data: { id: string; data: Category; updateMask: string },
    options?: RequestOptions,
  ): Promise<{ category: Category }> => {
    return assetApi.put<{ category: Category }>(
      `/categories/${id}`,
      data,
      options,
    );
  },

  delete: async (id: string, options?: RequestOptions): Promise<void> => {
    return assetApi.delete(`/categories/${id}`, options);
  },

  getTree: async (
    params?: { rootId?: string; maxDepth?: number },
    options?: RequestOptions,
  ): Promise<GetCategoryTreeResponse> => {
    return assetApi.get<GetCategoryTreeResponse>(
      `/categories/tree${buildQuery(params || {})}`,
      options,
    );
  },
};

// ==================== Asset Service ====================

export const AssetService = {
  list: async (
    params?: {
      query?: string;
      status?: AssetStatus;
      categoryId?: string;
      supplierId?: string;
      locationId?: string;
      userId?: number;
      page?: number;
      pageSize?: number;
      noPaging?: boolean;
    },
    options?: RequestOptions,
  ): Promise<ListAssetsResponse> => {
    return assetApi.get<ListAssetsResponse>(
      `/assets${buildQuery(params || {})}`,
      options,
    );
  },

  get: async (
    id: string,
    options?: RequestOptions,
  ): Promise<{ asset: Asset }> => {
    return assetApi.get<{ asset: Asset }>(`/assets/${id}`, options);
  },

  create: async (
    data: Partial<Asset>,
    options?: RequestOptions,
  ): Promise<{ asset: Asset }> => {
    return assetApi.post<{ asset: Asset }>('/assets', data, options);
  },

  update: async (
    id: string,
    data: { id: string; data: Asset; updateMask: string },
    options?: RequestOptions,
  ): Promise<{ asset: Asset }> => {
    return assetApi.put<{ asset: Asset }>(`/assets/${id}`, data, options);
  },

  delete: async (id: string, options?: RequestOptions): Promise<void> => {
    return assetApi.delete(`/assets/${id}`, options);
  },

  assign: async (
    id: string,
    data: { userId: number; notes?: string },
    options?: RequestOptions,
  ): Promise<{ asset: Asset }> => {
    return assetApi.post<{ asset: Asset }>(
      `/assets/${id}/assign`,
      data,
      options,
    );
  },

  unassign: async (
    id: string,
    data: { locationId?: string; notes?: string },
    options?: RequestOptions,
  ): Promise<{ asset: Asset }> => {
    return assetApi.post<{ asset: Asset }>(
      `/assets/${id}/unassign`,
      data,
      options,
    );
  },

  getAssignmentHistory: async (
    id: string,
    options?: RequestOptions,
  ): Promise<ListAssignmentsResponse> => {
    return assetApi.get<ListAssignmentsResponse>(
      `/assets/${id}/assignments`,
      options,
    );
  },

  uploadPhoto: async (
    id: string,
    data: { content: string; fileName: string },
    options?: RequestOptions,
  ): Promise<{ asset: Asset }> => {
    return assetApi.post<{ asset: Asset }>(
      `/assets/${id}/photo`,
      data,
      options,
    );
  },

  deletePhoto: async (id: string, options?: RequestOptions): Promise<void> => {
    return assetApi.delete(`/assets/${id}/photo`, options);
  },

  listDocuments: async (
    id: string,
    options?: RequestOptions,
  ): Promise<ListDocumentsResponse> => {
    return assetApi.get<ListDocumentsResponse>(
      `/assets/${id}/documents`,
      options,
    );
  },

  uploadDocument: async (
    id: string,
    data: { fileName: string; content: string; description?: string },
    options?: RequestOptions,
  ): Promise<{ document: AssetDocument }> => {
    return assetApi.post<{ document: AssetDocument }>(
      `/assets/${id}/documents`,
      data,
      options,
    );
  },

  deleteDocument: async (
    assetId: string,
    documentId: string,
    options?: RequestOptions,
  ): Promise<void> => {
    return assetApi.delete(
      `/assets/${assetId}/documents/${documentId}`,
      options,
    );
  },

  downloadDocument: async (
    assetId: string,
    documentId: string,
    options?: RequestOptions,
  ): Promise<{ content: string; fileName: string; mimeType: string }> => {
    return assetApi.get(
      `/assets/${assetId}/documents/${documentId}/download`,
      options,
    );
  },

  inventorySyncPreview: async (
    options?: RequestOptions,
  ): Promise<InventorySyncPreviewResponse> => {
    return assetApi.post<InventorySyncPreviewResponse>(
      '/assets/inventory-sync/preview',
      {},
      options,
    );
  },

  inventorySyncExecute: async (
    data?: { selectedHostnames?: string[] },
    options?: RequestOptions,
  ): Promise<InventorySyncExecuteResponse> => {
    return assetApi.post<InventorySyncExecuteResponse>(
      '/assets/inventory-sync/execute',
      data || {},
      options,
    );
  },
};

// ==================== Consumable Service ====================

export const ConsumableService = {
  list: async (
    params?: {
      query?: string;
      categoryId?: string;
      supplierId?: string;
      locationId?: string;
      page?: number;
      pageSize?: number;
      noPaging?: boolean;
    },
    options?: RequestOptions,
  ): Promise<ListConsumablesResponse> => {
    return assetApi.get<ListConsumablesResponse>(
      `/consumables${buildQuery(params || {})}`,
      options,
    );
  },

  get: async (
    id: string,
    options?: RequestOptions,
  ): Promise<{ consumable: Consumable }> => {
    return assetApi.get<{ consumable: Consumable }>(
      `/consumables/${id}`,
      options,
    );
  },

  create: async (
    data: Partial<Consumable>,
    options?: RequestOptions,
  ): Promise<{ consumable: Consumable }> => {
    return assetApi.post<{ consumable: Consumable }>(
      '/consumables',
      data,
      options,
    );
  },

  update: async (
    id: string,
    data: { id: string; data: Consumable; updateMask: string },
    options?: RequestOptions,
  ): Promise<{ consumable: Consumable }> => {
    return assetApi.put<{ consumable: Consumable }>(
      `/consumables/${id}`,
      data,
      options,
    );
  },

  delete: async (id: string, options?: RequestOptions): Promise<void> => {
    return assetApi.delete(`/consumables/${id}`, options);
  },

  listDocuments: async (
    id: string,
    options?: RequestOptions,
  ): Promise<ListDocumentsResponse> => {
    return assetApi.get<ListDocumentsResponse>(
      `/consumables/${id}/documents`,
      options,
    );
  },

  uploadDocument: async (
    id: string,
    data: { fileName: string; content: string; description?: string },
    options?: RequestOptions,
  ): Promise<{ document: AssetDocument }> => {
    return assetApi.post<{ document: AssetDocument }>(
      `/consumables/${id}/documents`,
      data,
      options,
    );
  },

  deleteDocument: async (
    consumableId: string,
    documentId: string,
    options?: RequestOptions,
  ): Promise<void> => {
    return assetApi.delete(
      `/consumables/${consumableId}/documents/${documentId}`,
      options,
    );
  },

  downloadDocument: async (
    consumableId: string,
    documentId: string,
    options?: RequestOptions,
  ): Promise<{ content: string; fileName: string; mimeType: string }> => {
    return assetApi.get(
      `/consumables/${consumableId}/documents/${documentId}/download`,
      options,
    );
  },
};

// ==================== License Service ====================

export const LicenseService = {
  list: async (
    params?: {
      query?: string;
      supplierId?: string;
      status?: LicenseStatus;
      page?: number;
      pageSize?: number;
      noPaging?: boolean;
    },
    options?: RequestOptions,
  ): Promise<ListLicensesResponse> => {
    return assetApi.get<ListLicensesResponse>(
      `/licenses${buildQuery(params || {})}`,
      options,
    );
  },

  get: async (
    id: string,
    options?: RequestOptions,
  ): Promise<{ license: License }> => {
    return assetApi.get<{ license: License }>(`/licenses/${id}`, options);
  },

  create: async (
    data: Partial<License>,
    options?: RequestOptions,
  ): Promise<{ license: License }> => {
    return assetApi.post<{ license: License }>('/licenses', data, options);
  },

  update: async (
    id: string,
    data: { id: string; data: License; updateMask: string },
    options?: RequestOptions,
  ): Promise<{ license: License }> => {
    return assetApi.put<{ license: License }>(
      `/licenses/${id}`,
      data,
      options,
    );
  },

  delete: async (id: string, options?: RequestOptions): Promise<void> => {
    return assetApi.delete(`/licenses/${id}`, options);
  },

  listDocuments: async (
    id: string,
    options?: RequestOptions,
  ): Promise<ListDocumentsResponse> => {
    return assetApi.get<ListDocumentsResponse>(
      `/licenses/${id}/documents`,
      options,
    );
  },

  uploadDocument: async (
    id: string,
    data: { fileName: string; content: string; description?: string },
    options?: RequestOptions,
  ): Promise<{ document: AssetDocument }> => {
    return assetApi.post<{ document: AssetDocument }>(
      `/licenses/${id}/documents`,
      data,
      options,
    );
  },

  deleteDocument: async (
    licenseId: string,
    documentId: string,
    options?: RequestOptions,
  ): Promise<void> => {
    return assetApi.delete(
      `/licenses/${licenseId}/documents/${documentId}`,
      options,
    );
  },

  downloadDocument: async (
    licenseId: string,
    documentId: string,
    options?: RequestOptions,
  ): Promise<{ content: string; fileName: string; mimeType: string }> => {
    return assetApi.get(
      `/licenses/${licenseId}/documents/${documentId}/download`,
      options,
    );
  },
};

// ==================== Insurance Policy Service ====================

export const InsurancePolicyService = {
  list: async (
    params?: {
      query?: string;
      status?: InsurancePolicyStatus;
      coverageType?: CoverageType;
      page?: number;
      pageSize?: number;
      noPaging?: boolean;
    },
    options?: RequestOptions,
  ): Promise<ListInsurancePoliciesResponse> => {
    return assetApi.get<ListInsurancePoliciesResponse>(
      `/insurance-policies${buildQuery(params || {})}`,
      options,
    );
  },

  get: async (
    id: string,
    options?: RequestOptions,
  ): Promise<{ insurancePolicy: InsurancePolicy }> => {
    return assetApi.get<{ insurancePolicy: InsurancePolicy }>(
      `/insurance-policies/${id}`,
      options,
    );
  },

  create: async (
    data: Partial<InsurancePolicy>,
    options?: RequestOptions,
  ): Promise<{ insurancePolicy: InsurancePolicy }> => {
    return assetApi.post<{ insurancePolicy: InsurancePolicy }>(
      '/insurance-policies',
      data,
      options,
    );
  },

  update: async (
    id: string,
    data: {
      id: string;
      data: InsurancePolicy;
      updateMask: string;
    },
    options?: RequestOptions,
  ): Promise<{ insurancePolicy: InsurancePolicy }> => {
    return assetApi.put<{ insurancePolicy: InsurancePolicy }>(
      `/insurance-policies/${id}`,
      data,
      options,
    );
  },

  delete: async (id: string, options?: RequestOptions): Promise<void> => {
    return assetApi.delete(`/insurance-policies/${id}`, options);
  },

  listAssets: async (
    id: string,
    options?: RequestOptions,
  ): Promise<ListPolicyAssetsResponse> => {
    return assetApi.get<ListPolicyAssetsResponse>(
      `/insurance-policies/${id}/assets`,
      options,
    );
  },

  addAsset: async (
    id: string,
    data: { assetId: string; coveredValue?: number; notes?: string },
    options?: RequestOptions,
  ): Promise<{ policyAsset: PolicyAsset }> => {
    return assetApi.post<{ policyAsset: PolicyAsset }>(
      `/insurance-policies/${id}/assets`,
      data,
      options,
    );
  },

  removeAsset: async (
    id: string,
    assetId: string,
    options?: RequestOptions,
  ): Promise<void> => {
    return assetApi.delete(
      `/insurance-policies/${id}/assets/${assetId}`,
      options,
    );
  },
};

// ==================== System Service ====================

export const SystemService = {
  healthCheck: async (
    options?: RequestOptions,
  ): Promise<HealthCheckResponse> => {
    return assetApi.get<HealthCheckResponse>('/health', options);
  },

  getStats: async (
    options?: RequestOptions,
  ): Promise<{ stats: DashboardStats }> => {
    return assetApi.get<{ stats: DashboardStats }>('/stats', options);
  },
};
