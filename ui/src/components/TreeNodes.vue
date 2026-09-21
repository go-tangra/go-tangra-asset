<script setup lang="ts">
// Recursive tree renderer for categories and locations.
export interface TreeItem {
  id: string
  name: string
  path?: string
  asset_count: number
  child_count: number
  children?: TreeItem[]
}
defineProps<{ items: TreeItem[]; depth?: number }>()
defineEmits<{ (e: 'edit', item: TreeItem): void; (e: 'remove', item: TreeItem): void; (e: 'add-child', item: TreeItem): void }>()
</script>

<template>
  <div>
    <div v-for="it in items" :key="it.id">
      <div class="d-flex align-center py-1" :style="{ paddingLeft: (depth ?? 0) * 24 + 'px' }">
        <v-icon :icon="it.children && it.children.length ? 'mdi-folder-outline' : 'mdi-subdirectory-arrow-right'" size="small" class="me-2" />
        <span class="font-weight-medium">{{ it.name }}</span>
        <v-chip size="x-small" variant="tonal" class="ms-2">{{ it.asset_count }} assets</v-chip>
        <v-spacer />
        <v-btn icon="mdi-plus" size="x-small" variant="text" title="Add child" @click="$emit('add-child', it)" />
        <v-btn icon="mdi-pencil-outline" size="x-small" variant="text" @click="$emit('edit', it)" />
        <v-btn icon="mdi-delete-outline" size="x-small" variant="text" :disabled="it.child_count > 0 || it.asset_count > 0" @click="$emit('remove', it)" />
      </div>
      <TreeNodes v-if="it.children && it.children.length" :items="it.children" :depth="(depth ?? 0) + 1" @edit="$emit('edit', $event)" @remove="$emit('remove', $event)" @add-child="$emit('add-child', $event)" />
    </div>
  </div>
</template>
