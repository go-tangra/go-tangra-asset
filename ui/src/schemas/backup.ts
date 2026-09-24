import { z } from 'zod'

export const BACKUP_MODES = ['skip', 'overwrite'] as const

/** Import form: a JSON file plus the conflict mode. The file is parsed before the request. */
export const backupImportSchema = z.object({
  file: z.instanceof(File, { message: 'Choose a backup file.' }).refine((f) => f.size <= 50 * 1024 * 1024, 'The file is larger than 50 MiB.'),
  mode: z.enum(BACKUP_MODES),
})
export type BackupImportInput = z.output<typeof backupImportSchema>
