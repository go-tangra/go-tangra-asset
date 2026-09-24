<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { UiPage, UiAlert, UiCard, UiButton, UiTree, UiEmptyState, UiToolbar, UiBadge, UiRecordDrawer, useConfirm, type TreeNode } from '@go-tangra/ui'
import { zodToFields } from '@go-tangra/ui/forms'
import { useOrg } from '@/stores/org'
import { describe } from '@/api/client'
import { categorySchema } from '@/schemas'
import type { Category } from '@/api/types'

const org = useOrg()
const confirm = useConfirm()
const dialog = ref(false)
const editing = ref<Category | null>(null)
const parentFor = ref('')
const selectedId = ref('')
const error = ref('')

onMounted(() => void org.loadCategories())

const toNode = (c: Category): TreeNode => ({ id: c.id, label: c.name, icon: c.icon || (c.children?.length ? 'mdi-folder-outline' : 'mdi-tag-outline'), badge: String(c.asset_count), children: (c.children ?? []).map(toNode) })
const tree = computed<TreeNode[]>(() => org.categoryTree.map(toNode))
const selected = computed(() => org.categories.find((c) => c.id === selectedId.value) ?? null)

const fields = computed(() =>
  zodToFields(categorySchema, {
    parent_id: { type: 'select', options: org.categories.filter((c) => c.id !== editing.value?.id).map((c) => ({ title: c.name, value: c.id })) },
    icon: { label: 'Icon (mdi-…)' },
  }),
)

function add(parent?: Category | null): void {
  editing.value = null
  parentFor.value = parent?.id ?? ''
  dialog.value = true
}
function edit(c: Category): void {
  editing.value = c
  dialog.value = true
}
async function remove(c: Category): Promise<void> {
  if (!(await confirm.ask({ title: `Delete ${c.name}?`, danger: true, confirmLabel: 'Delete' }))) return
  error.value = ''
  try {
    await org.removeCategory(c.id)
    selectedId.value = ''
    await org.loadCategories()
  } catch (e) {
    error.value = describe(e)
  }
}
const initial = computed(() => (editing.value ? { ...editing.value } : { parent_id: parentFor.value }))
const submit = (v: Record<string, unknown>) => (editing.value ? org.updateCategory(editing.value.id, v) : org.createCategory(v))
</script>

<template>
  <UiPage title="Categories">
    <template #actions>
      <UiButton icon="mdi-plus" @click="add()">New category</UiButton>
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="org.loadCategories()" />
    </template>
    <UiAlert v-if="error || org.error" kind="error" class="mb-3">{{ error || org.error }}</UiAlert>
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
      <UiCard class="lg:col-span-2">
        <UiEmptyState v-if="tree.length === 0" title="No categories yet" />
        <UiTree v-else v-model:selected="selectedId" :items="tree" />
      </UiCard>
      <UiCard :title="selected ? selected.name : 'Category'">
        <p v-if="!selected" class="text-sm text-base-content/70">Select a category to edit it or add a child.</p>
        <template v-else>
          <p v-if="selected.description" class="mb-2 text-sm">{{ selected.description }}</p>
          <UiToolbar class="mb-3"><UiBadge>{{ selected.asset_count }} assets</UiBadge><UiBadge>{{ selected.child_count }} children</UiBadge></UiToolbar>
          <UiToolbar>
            <UiButton size="sm" variant="soft" icon="mdi-plus" @click="add(selected)">Add child</UiButton>
            <UiButton size="sm" variant="soft" icon="mdi-pencil-outline" @click="edit(selected)">Edit</UiButton>
            <UiButton size="sm" variant="text" color="error" icon="mdi-delete-outline" :disabled="selected.child_count > 0 || selected.asset_count > 0" @click="remove(selected)">Delete</UiButton>
          </UiToolbar>
        </template>
      </UiCard>
    </div>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit category' : 'New category'" :schema="categorySchema" :fields="fields" :initial="initial" :submit="submit" @saved="org.loadCategories()" />
  </UiPage>
</template>
