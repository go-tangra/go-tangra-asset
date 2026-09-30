<script setup lang="ts">
// Full-text search over the documents attached to assets, consumables and
// licenses (stored in paperless, feature 030).
import { ref } from 'vue'
import { UiAlert, UiButton, UiCard, UiEmptyState, UiInput, UiPage } from '@go-tangra/ui'
import { api, describe } from '@/api/client'
import { useDocuments } from '@/stores/documents'
import type { DocumentHit } from '@/api/types'

const docs = useDocuments()
const query = ref('')
const hits = ref<DocumentHit[] | null>(null)
const loading = ref(false)
const error = ref('')

async function search(): Promise<void> {
  const q = query.value.trim()
  if (!q) return
  loading.value = true
  error.value = ''
  try {
    const res = await api<{ items: DocumentHit[] }>('GET', 'documents/search', undefined, { query: { q, limit: 50 } })
    hits.value = res.items ?? []
  } catch (e) {
    error.value = describe(e)
    hits.value = null
  } finally {
    loading.value = false
  }
}

const kinds: Record<string, string> = { asset: 'Asset', consumable: 'Consumable', license: 'License' }
function entityLink(h: DocumentHit): string {
  const d = h.document
  if (d.entity_type === 'asset') return '/asset/item/' + d.entity_id
  return d.entity_type === 'consumable' ? '/asset/consumables' : '/asset/licenses'
}
const size = (n: number): string => (n >= 1 << 20 ? (n / (1 << 20)).toFixed(1) + ' MiB' : Math.max(1, Math.round(n / 1024)) + ' KiB')
</script>

<template>
  <UiPage title="Document search" subtitle="Search the text of documents attached to assets, consumables and licenses">
    <UiCard class="mb-4">
      <form class="flex flex-wrap items-end gap-2" @submit.prevent="search">
        <div class="min-w-64 flex-1">
          <UiInput id="doc-search" v-model="query" label="Search text" placeholder="e.g. invoice number, serial, warranty" data-test="doc-search" />
        </div>
        <UiButton type="submit" icon="mdi-magnify" :loading="loading" :disabled="!query.trim()" data-test="doc-search-go">Search</UiButton>
      </form>
    </UiCard>
    <UiAlert v-if="error" kind="error" class="mb-3">{{ error }}</UiAlert>
    <template v-if="hits">
      <UiEmptyState v-if="!hits.length" title="No documents found" text="No attached document contains that text." icon="mdi-file-search-outline" />
      <ul v-else class="flex flex-col gap-2" data-test="doc-hits">
        <li v-for="h in hits" :key="h.document.id">
          <UiCard>
            <div class="flex flex-wrap items-start justify-between gap-2">
              <div class="min-w-0">
                <a class="link font-medium" :href="docs.downloadUrl(h.document.entity_type, h.document.entity_id, h.document.id)" target="_blank" rel="noopener">{{ h.document.file_name }}</a>
                <p class="text-sm text-base-content/70">
                  {{ kinds[h.document.entity_type] ?? h.document.entity_type }} ·
                  <RouterLink class="link" :to="entityLink(h)">open</RouterLink> ·
                  {{ size(h.document.file_size) }} · {{ new Date(h.document.created_at).toLocaleDateString() }}
                </p>
                <p v-if="h.snippet" class="mt-1 text-sm">{{ h.snippet }}</p>
              </div>
            </div>
          </UiCard>
        </li>
      </ul>
    </template>
    <UiEmptyState v-else title="Search documents" text="Documents are indexed by the paperless module, so the text of PDFs and office files can be searched." icon="mdi-file-search-outline" />
  </UiPage>
</template>
