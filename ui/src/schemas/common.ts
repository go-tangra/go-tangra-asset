// Field primitives shared by the asset schemas (on top of the kit's).
import { z } from 'zod'
import { optionalString } from '@freya/ui/forms'

/** Optional reference to another record (select fields); blank → undefined. */
export const ref = optionalString(64)

/** Optional enum: blank/null → undefined. */
export const optionalEnum = <const T extends readonly [string, ...string[]]>(values: T) =>
  z.preprocess((v) => (v === '' || v === null ? undefined : v), z.enum(values).optional())
