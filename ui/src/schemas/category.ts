import { z } from 'zod'
import { nonEmpty, optionalString, tagMap } from '@freya/ui/forms'
import { ref } from './common'

export const categorySchema = z.object({
  name: nonEmpty(200),
  parent_id: ref,
  icon: optionalString(64).pipe(z.string().regex(/^mdi-[a-z0-9-]+$/, 'Use an mdi-… icon name.').optional()),
  description: optionalString(2000),
  tags: tagMap(64),
})
export type CategoryInput = z.output<typeof categorySchema>
