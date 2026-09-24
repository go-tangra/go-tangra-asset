import { defineStore } from 'pinia'
import { api, upload, fileUrl } from '@/api/client'
import type { Asset, Document } from '@/api/types'

const ROUTE: Record<string, string> = { asset: 'assets', consumable: 'consumables', license: 'licenses' }

// Photos and polymorphic documents (asset | consumable | license).
export const useDocuments = defineStore('asset-documents', () => {
  const base = (type: string, id: string) => ROUTE[type] + '/' + id + '/documents'
  const list = async (type: string, id: string): Promise<Document[]> => (await api<{ items: Document[] }>('GET', base(type, id))).items ?? []
  const add = (type: string, id: string, file: File, description = '') => upload<Document>(base(type, id), file, { description })
  const remove = (type: string, id: string, docId: string) => api<void>('DELETE', base(type, id) + '/' + docId)
  const downloadUrl = (type: string, id: string, docId: string) => fileUrl(base(type, id) + '/' + docId + '/download')
  const uploadPhoto = (assetId: string, file: File) => upload<Asset>('assets/' + assetId + '/photo', file)
  const photoUrl = (assetId: string, bust = '') => fileUrl('assets/' + assetId + '/photo') + (bust ? '?v=' + encodeURIComponent(bust) : '')
  return { list, add, remove, downloadUrl, uploadPhoto, photoUrl }
})
