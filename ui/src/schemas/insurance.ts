import { z } from 'zod'
import { nonEmpty, optionalString, isoDate, money, dateRange } from '@go-tangra/ui/forms'
import { optionalEnum } from './common'

export const POLICY_STATUSES = ['active', 'expired', 'cancelled'] as const
export const COVERAGE_TYPES = ['all_risk', 'fire_theft', 'liability', 'equipment_breakdown', 'cyber'] as const

export const insurancePolicySchema = dateRange(
  z.object({
    name: nonEmpty(200),
    policy_number: nonEmpty(100),
    provider: optionalString(200),
    coverage_type: optionalEnum(COVERAGE_TYPES),
    status: optionalEnum(POLICY_STATUSES),
    premium_amount: money,
    deductible: money,
    coverage_limit: money,
    valid_from: isoDate,
    valid_to: isoDate,
    notes: optionalString(4000),
  }),
  'valid_from',
  'valid_to',
)
export type InsurancePolicyInput = z.output<typeof insurancePolicySchema>
