import { z } from 'zod'
import { nonEmpty, optionalString, isoDate, money, positiveInt, tagMap } from '@freya/ui/forms'
import { ref, optionalEnum } from './common'

export const ASSET_STATUSES = ['deployable', 'broken', 'archived'] as const

/** POST/PUT /assets payload (api/openapi/asset.yaml AssetInput). */
export const assetSchema = z.object({
  name: nonEmpty(200),
  asset_tag: optionalString(64),
  serial: optionalString(200),
  model_name: optionalString(200),
  model_number: optionalString(200),
  category_id: ref,
  supplier_id: ref,
  location_id: ref,
  status: optionalEnum(ASSET_STATUSES),
  purchase_date: isoDate,
  purchase_cost: money,
  order_number: optionalString(100),
  warranty_months: positiveInt,
  useful_life_years: positiveInt.pipe(z.number().max(100)),
  salvage_value: money,
  depreciation_rate: z.preprocess((v) => (v === '' || v === null || v === undefined ? 0 : v), z.coerce.number().min(0).max(1)).meta({ kind: 'number', optional: true }),
  notes: optionalString(4000),
  tags: tagMap(64),
})
export type AssetInput = z.output<typeof assetSchema>

export const assignSchema = z.object({
  user_id: nonEmpty(64),
  notes: optionalString(1000),
})
export const unassignSchema = z.object({
  location_id: ref,
  notes: optionalString(1000),
})
