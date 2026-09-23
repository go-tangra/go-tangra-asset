import { z } from 'zod'
import { nonEmpty, optionalString, email } from '@freya/ui/forms'
import { optionalEnum } from './common'

export const SUPPLIER_STATUSES = ['active', 'inactive'] as const

export const supplierSchema = z.object({
  name: nonEmpty(200),
  code: optionalString(64),
  status: optionalEnum(SUPPLIER_STATUSES),
  website: optionalString(500).pipe(z.url().optional()),
  address: optionalString(500),
  city: optionalString(200),
  state: optionalString(200),
  country: optionalString(200),
  postal_code: optionalString(32),
  contact_person: optionalString(200),
  telephone: optionalString(64),
  email: optionalString(320).pipe(email.optional()).meta({ kind: 'email', optional: true }),
  notes: optionalString(4000),
})
export type SupplierInput = z.output<typeof supplierSchema>
