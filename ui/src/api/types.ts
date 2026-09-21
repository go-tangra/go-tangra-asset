// Plain response shapes of the asset API (see api/openapi/asset.yaml).

export interface Asset {
  id: string
  tenant_id: string
  asset_tag: string
  name?: string
  serial?: string
  model_name?: string
  model_number?: string
  category_id?: string
  supplier_id?: string
  location_id?: string
  user_id?: string
  assignee_name?: string
  status: 'deployable' | 'assigned' | 'broken' | 'archived'
  has_photo: boolean
  warranty_months?: number
  purchase_date?: string
  order_number?: string
  purchase_cost?: number
  notes?: string
  salvage_value?: number
  useful_life_years?: number
  depreciation_rate?: number
  tags?: Record<string, string>
  book_value?: number
  created_by?: string
  updated_by?: string
  created_at: string
  updated_at: string
}

export interface AssetInput {
  asset_tag?: string
  name: string
  serial?: string
  model_name?: string
  model_number?: string
  category_id?: string
  supplier_id?: string
  location_id?: string
  status?: string
  warranty_months?: number
  purchase_date?: string | null
  order_number?: string
  purchase_cost?: number
  notes?: string
  salvage_value?: number
  useful_life_years?: number
  depreciation_rate?: number
  tags?: Record<string, string>
}

export interface Assignment {
  id: string
  asset_id: string
  asset_name?: string
  user_id?: string
  user_name?: string
  action: 'assigned' | 'unassigned' | 'transferred'
  assigned_at: string
  returned_at?: string
  assigned_by?: string
  notes?: string
}

export interface Document {
  id: string
  entity_type: 'asset' | 'consumable' | 'license'
  entity_id: string
  file_name: string
  file_size: number
  mime_type?: string
  checksum?: string
  description?: string
  uploaded_by?: string
  created_at: string
}

export interface Category {
  id: string
  name: string
  description?: string
  parent_id?: string
  icon?: string
  tags?: Record<string, string>
  asset_count: number
  child_count: number
  children?: Category[]
}

export interface Supplier {
  id: string
  name: string
  code?: string
  address?: string
  city?: string
  state?: string
  country?: string
  postal_code?: string
  contact_person?: string
  telephone?: string
  email?: string
  website?: string
  notes?: string
  status: 'active' | 'inactive'
}

export interface Location {
  id: string
  name: string
  code?: string
  description?: string
  parent_id?: string
  path?: string
  address?: string
  city?: string
  state?: string
  country?: string
  postal_code?: string
  contact?: string
  phone?: string
  email?: string
  status: 'active' | 'planned' | 'decommissioned'
  child_count: number
  asset_count: number
  children?: Location[]
}

export interface Consumable {
  id: string
  name: string
  description?: string
  category_id?: string
  supplier_id?: string
  location_id?: string
  model_name?: string
  model_number?: string
  amount: number
  min_amount: number
  purchase_date?: string
  purchase_cost?: number
  order_number?: string
  notes?: string
  low_stock: boolean
}

export interface License {
  id: string
  name: string
  supplier_id?: string
  purchase_date?: string
  purchase_cost?: number
  order_number?: string
  valid_from?: string
  valid_to?: string
  notes?: string
  status: 'active' | 'expired' | 'suspended'
}

export interface InsurancePolicy {
  id: string
  name: string
  policy_number: string
  provider?: string
  coverage_type?: string
  premium_amount?: number
  deductible?: number
  coverage_limit?: number
  valid_from?: string
  valid_to?: string
  status: 'active' | 'expired' | 'cancelled'
  notes?: string
  asset_count: number
}

export interface PolicyAsset {
  id: string
  policy_id: string
  asset_id: string
  covered_value?: number
  notes?: string
  asset_tag?: string
  asset_name?: string
  model_name?: string
  created_at: string
}

export interface User {
  id: string
  display_name: string
  avatar_url?: string
}

export interface SyncChange {
  host_id: string
  hostname: string
  serial?: string
  action: 'create' | 'update' | 'unchanged'
  asset_id?: string
  asset_tag?: string
  changes?: Record<string, { old: string; new: string }>
}

export interface SyncPreview {
  hosts: number
  create: number
  update: number
  unchanged: number
  changes: SyncChange[]
}

export interface SyncResult {
  created: number
  updated: number
  skipped: number
  selected: number
  errors: string[]
  changes: SyncChange[]
}

export interface Dashboard {
  total_assets: number
  assets_by_status: Record<string, number>
  total_consumables: number
  total_suppliers: number
  total_categories: number
  total_locations: number
  total_licenses: number
  total_insurance_policies: number
  total_cost: number
  total_depreciated_value: number
  expiring_soon: number
  low_stock: number
  warranty_expiring_soon: number
  licenses_expiring_soon: number
  insurance_expiring_soon: number
  assigned_assets: number
}
