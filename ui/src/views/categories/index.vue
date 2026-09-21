<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useOrg } from '@/stores/org'
import { describe } from '@/api/client'
import RecordDialog from '@/components/RecordDialog.vue'
import TreeNodes from '@/components/TreeNodes.vue'
import type { TreeItem } from '@/components/TreeNodes.vue'
import type { Field } from '@/components/RecordDialog.vue'
import type { Category } from '@/api/types'

const org = useOrg()
const dialog = ref(false)
const editing = ref<Category | null>(null)
const parentFor = ref<string>('')
const error = ref('')

onMounted(() => void org.loadCategories())

const fields = computed<Field[]>(() => [
  { key: 'name', label: 'Name', required: true },
  { key: 'parent_id', label: 'Parent', type: 'select', options: org.categories.filter((c) => c.id !== editing.value?.id).map((c) => ({ title: c.name, value: c.id })) },
  { key: 'icon', label: 'Icon (mdi-…)' },
  { key: 'description', label: 'Description', type: 'textarea', cols: 12 },
])

function add(parent?: TreeItem): void {
  editing.value = null
  parentFor.value = parent?.id ?? ''
  dialog.value = true
}
function edit(item: TreeItem): void {
  editing.value = org.categories.find((c) => c.id === item.id) ?? null
  dialog.value = true
}
async function remove(item: TreeItem): Promise<void> {
  error.value = ''
  try {
    await org.removeCategory(item.id)
    await org.loadCategories()
  } catch (e) {
    error.value = describe(e)
  }
}
const initial = computed(() => (editing.value ? { ...editing.value } : { parent_id: parentFor.value }))
const submit = (v: Record<string, unknown>) => (editing.value ? org.updateCategory(editing.value.id, v) : org.createCategory(v))
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Categories</h1>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-plus" class="me-2" @click="add()">New category</v-btn>
      <v-btn variant="text" icon="mdi-refresh" @click="org.loadCategories()" />
    </div>
    <v-alert v-if="error || org.error" type="error" variant="tonal" density="compact" class="mb-3">{{ error || org.error }}</v-alert>
    <v-card>
      <v-card-text>
        <div v-if="org.categoryTree.length === 0" class="text-medium-emphasis">No categories yet.</div>
        <TreeNodes :items="org.categoryTree as TreeItem[]" @edit="edit" @remove="remove" @add-child="add" />
      </v-card-text>
    </v-card>
    <RecordDialog v-model="dialog" :title="editing ? 'Edit category' : 'New category'" :fields="fields" :initial="initial" :submit="submit" @saved="org.loadCategories()" />
  </div>
</template>
