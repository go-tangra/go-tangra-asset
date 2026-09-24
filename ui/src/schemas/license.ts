import { z } from 'zod'
import { nonEmpty, optionalString, isoDate, money, dateRange } from '@go-tangra/ui/forms'
import { ref, optionalEnum } from './common'

export const LICENSE_STATUSES = ['active', 'expired', 'suspended'] as const

export const licenseSchema = dateRange(
  z.object({
    name: nonEmpty(200),
    supplier_id: ref,
    status: optionalEnum(LICENSE_STATUSES),
    valid_from: isoDate,
    valid_to: isoDate,
    purchase_date: isoDate,
    purchase_cost: money,
    order_number: optionalString(100),
    notes: optionalString(4000),
  }),
  'valid_from',
  'valid_to',
)
export type LicenseInput = z.output<typeof licenseSchema>
