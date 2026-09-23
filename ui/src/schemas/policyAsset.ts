import { z } from 'zod'
import { nonEmpty, optionalString, money } from '@freya/ui/forms'

export const policyAssetSchema = z.object({
  asset_id: nonEmpty(64),
  covered_value: money,
  notes: optionalString(1000),
})
export type PolicyAssetInput = z.output<typeof policyAssetSchema>
