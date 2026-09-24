import { z } from 'zod'
import { nonEmpty, optionalString, isoDate, money, positiveInt } from '@go-tangra/ui/forms'
import { ref } from './common'

export const consumableSchema = z.object({
  name: nonEmpty(200),
  model_name: optionalString(200),
  model_number: optionalString(200),
  amount: positiveInt,
  min_amount: positiveInt,
  category_id: ref,
  supplier_id: ref,
  location_id: ref,
  purchase_date: isoDate,
  purchase_cost: money,
  order_number: optionalString(100),
  description: optionalString(2000),
  notes: optionalString(4000),
})
export type ConsumableInput = z.output<typeof consumableSchema>
