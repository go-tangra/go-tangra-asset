import { z } from 'zod'
import { nonEmpty, optionalString, email } from '@go-tangra/ui/forms'
import { ref, optionalEnum } from './common'

export const LOCATION_STATUSES = ['active', 'planned', 'decommissioned'] as const

export const locationSchema = z.object({
  name: nonEmpty(200),
  code: optionalString(64),
  parent_id: ref,
  status: optionalEnum(LOCATION_STATUSES),
  address: optionalString(500),
  city: optionalString(200),
  state: optionalString(200),
  country: optionalString(200),
  postal_code: optionalString(32),
  contact: optionalString(200),
  phone: optionalString(64),
  email: optionalString(320).pipe(email.optional()).meta({ kind: 'email', optional: true }),
  description: optionalString(2000),
})
export type LocationInput = z.output<typeof locationSchema>
