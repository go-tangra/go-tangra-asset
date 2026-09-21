<script setup lang="ts">
// Documents attached to an asset, consumable or license: list, upload, download, delete.
import { onMounted, ref, watch } from 'vue'
import { useDocuments } from '@/stores/documents'
import { describe } from '@/api/client'
import type { Document } from '@/api/types'

const props = defineProps<{ entityType: 'asset' | 'consumable' | 'license'; entityId: string }>()
const docs = useDocuments()
const items = ref<Document[]>([])
const error = ref('')
const busy = ref(false)
const file = ref<File | null>(null)
const description = ref('')

async function reload(): Promise<void> {
  error.value = ''
  try {
    items.value = await docs.list(props.entityType, props.entityId)
  } catch (e) {
    error.value = describe(e)
  }
}
onMounted(reload)
watch(() => props.entityId, reload)

async function send(): Promise<void> {
  if (!file.value) return
  busy.value = true
  error.value = ''
  try {
    await docs.add(props.entityType, props.entityId, file.value, description.value)
    file.value = null
    description.value = ''
    await reload()
  } catch (e) {
    error.value = describe(e)
  } finally {
    busy.value = false
  }
}

async function remove(id: string): Promise<void> {
  error.value = ''
  try {
    await docs.remove(props.entityType, props.entityId, id)
    await reload()
  } catch (e) {
    error.value = describe(e)
  }
}

function size(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1048576) return (n / 1024).toFixed(1) + ' KB'
  return (n / 1048576).toFixed(1) + ' MB'
}
</script>

<template>
  <v-card variant="outlined">
    <v-card-title class="text-subtitle-1">Documents</v-card-title>
    <v-card-text>
      <v-alert v-if="error" type="error" variant="tonal" density="compact" class="mb-3">{{ error }}</v-alert>
      <v-row dense align="center">
        <v-col cols="12" md="6"><v-file-input v-model="file" label="File" density="comfortable" prepend-icon="mdi-paperclip" /></v-col>
        <v-col cols="12" md="4"><v-text-field v-model="description" label="Description" density="comfortable" /></v-col>
        <v-col cols="12" md="2"><v-btn color="primary" block :disabled="!file" :loading="busy" @click="send">Upload</v-btn></v-col>
      </v-row>
      <v-table density="compact">
        <thead>
          <tr><th>Name</th><th>Type</th><th>Size</th><th>Description</th><th>Uploaded</th><th /></tr>
        </thead>
        <tbody>
          <tr v-if="items.length === 0"><td colspan="6" class="text-medium-emphasis">No documents.</td></tr>
          <tr v-for="d in items" :key="d.id">
            <td><a :href="docs.downloadUrl(entityType, entityId, d.id)" target="_blank" rel="noopener">{{ d.file_name }}</a></td>
            <td>{{ d.mime_type || '—' }}</td>
            <td>{{ size(d.file_size) }}</td>
            <td>{{ d.description || '—' }}</td>
            <td>{{ new Date(d.created_at).toLocaleString() }}</td>
            <td class="text-right"><v-btn icon="mdi-delete-outline" size="small" variant="text" @click="remove(d.id)" /></td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>
</template>
