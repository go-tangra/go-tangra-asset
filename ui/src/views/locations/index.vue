<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { UiPage, UiAlert, UiCard, UiButton, UiTree, UiEmptyState, UiToolbar, UiBadge, UiStatusChip, UiKeyValueTable, UiRecordDrawer, useConfirm, type TreeNode } from '@go-tangra/ui'
import { zodToFields } from '@go-tangra/ui/forms'
import { useOrg } from '@/stores/org'
import { describe } from '@/api/client'
import { locationSchema, LOCATION_STATUSES } from '@/schemas'
import type { Location } from '@/api/types'

const org = useOrg()
const confirm = useConfirm()
const dialog = ref(false)
const editing = ref<Location | null>(null)
const parentFor = ref('')
const selectedId = ref('')
const error = ref('')

onMounted(() => void org.loadLocations())

const toNode = (l: Location): TreeNode => ({ id: l.id, label: l.name, icon: l.children?.length ? 'mdi-map-marker-multiple-outline' : 'mdi-map-marker-outline', badge: String(l.asset_count), children: (l.children ?? []).map(toNode) })
const tree = computed<TreeNode[]>(() => org.locationTree.map(toNode))
const selected = computed(() => org.locations.find((l) => l.id === selectedId.value) ?? null)

const fields = computed(() =>
  zodToFields(locationSchema, {
    parent_id: { type: 'select', options: org.locations.filter((l) => l.id !== editing.value?.id).map((l) => ({ title: l.path || l.name, value: l.id })) },
    status: { type: 'select', options: LOCATION_STATUSES.map((s) => ({ title: s, value: s })) },
    contact: { label: 'Contact (sealed)' },
    phone: { label: 'Phone (sealed)' },
    email: { label: 'E-mail (sealed)' },
  }),
)

function add(parent?: Location | null): void {
  editing.value = null
  parentFor.value = parent?.id ?? ''
  dialog.value = true
}
function edit(l: Location): void {
  editing.value = l
  dialog.value = true
}
async function remove(l: Location): Promise<void> {
  if (!(await confirm.ask({ title: `Delete ${l.name}?`, danger: true, confirmLabel: 'Delete' }))) return
  error.value = ''
  try {
    await org.removeLocation(l.id)
    selectedId.value = ''
    await org.loadLocations()
  } catch (e) {
    error.value = describe(e)
  }
}
const initial = computed(() => (editing.value ? { ...editing.value } : { parent_id: parentFor.value, status: 'active' }))
const submit = (v: Record<string, unknown>) => (editing.value ? org.updateLocation(editing.value.id, v) : org.createLocation(v))
</script>

<template>
  <UiPage title="Locations">
    <template #actions>
      <UiButton icon="mdi-plus" @click="add()">New location</UiButton>
      <UiButton variant="text" icon="mdi-refresh" icon-only label="Refresh" @click="org.loadLocations()" />
    </template>
    <UiAlert v-if="error || org.error" kind="error" class="mb-3">{{ error || org.error }}</UiAlert>
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
      <UiCard class="lg:col-span-2">
        <UiEmptyState v-if="tree.length === 0" title="No locations yet" />
        <UiTree v-else v-model:selected="selectedId" :items="tree" />
      </UiCard>
      <UiCard :title="selected ? selected.name : 'Location'">
        <p v-if="!selected" class="text-sm text-base-content/70">Select a location to edit it or add a child.</p>
        <template v-else>
          <UiToolbar class="mb-3"><UiStatusChip :status="selected.status" /><UiBadge>{{ selected.asset_count }} assets</UiBadge><UiBadge>{{ selected.child_count }} children</UiBadge></UiToolbar>
          <UiKeyValueTable class="mb-3" :items="[{ label: 'Path', value: selected.path }, { label: 'Code', value: selected.code }, { label: 'Address', value: [selected.address, selected.postal_code, selected.city, selected.country].filter(Boolean).join(', ') }, { label: 'Contact', value: selected.contact || selected.email || '(redacted)' }]" />
          <UiToolbar>
            <UiButton size="sm" variant="soft" icon="mdi-plus" @click="add(selected)">Add child</UiButton>
            <UiButton size="sm" variant="soft" icon="mdi-pencil-outline" @click="edit(selected)">Edit</UiButton>
            <UiButton size="sm" variant="text" color="error" icon="mdi-delete-outline" :disabled="selected.child_count > 0 || selected.asset_count > 0" @click="remove(selected)">Delete</UiButton>
          </UiToolbar>
        </template>
      </UiCard>
    </div>
    <UiRecordDrawer v-model="dialog" close-on-save :title="editing ? 'Edit location' : 'New location'" :schema="locationSchema" :fields="fields" :initial="initial" :submit="submit" size="lg" @saved="org.loadLocations()" />
  </UiPage>
</template>
