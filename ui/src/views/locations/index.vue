<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useOrg } from '@/stores/org'
import { describe } from '@/api/client'
import RecordDialog from '@/components/RecordDialog.vue'
import TreeNodes from '@/components/TreeNodes.vue'
import type { TreeItem } from '@/components/TreeNodes.vue'
import type { Field } from '@/components/RecordDialog.vue'
import type { Location } from '@/api/types'

const org = useOrg()
const dialog = ref(false)
const editing = ref<Location | null>(null)
const parentFor = ref('')
const error = ref('')

onMounted(() => void org.loadLocations())

const fields = computed<Field[]>(() => [
  { key: 'name', label: 'Name', required: true },
  { key: 'code', label: 'Code' },
  { key: 'parent_id', label: 'Parent', type: 'select', options: org.locations.filter((l) => l.id !== editing.value?.id).map((l) => ({ title: l.path || l.name, value: l.id })) },
  { key: 'status', label: 'Status', type: 'select', options: ['active', 'planned', 'decommissioned'].map((s) => ({ title: s, value: s })) },
  { key: 'address', label: 'Address' },
  { key: 'city', label: 'City' },
  { key: 'state', label: 'State' },
  { key: 'country', label: 'Country' },
  { key: 'postal_code', label: 'Postal code' },
  { key: 'contact', label: 'Contact (sealed)' },
  { key: 'phone', label: 'Phone (sealed)' },
  { key: 'email', label: 'E-mail (sealed)' },
  { key: 'description', label: 'Description', type: 'textarea', cols: 12 },
])

function add(parent?: TreeItem): void {
  editing.value = null
  parentFor.value = parent?.id ?? ''
  dialog.value = true
}
function edit(item: TreeItem): void {
  editing.value = org.locations.find((l) => l.id === item.id) ?? null
  dialog.value = true
}
async function remove(item: TreeItem): Promise<void> {
  error.value = ''
  try {
    await org.removeLocation(item.id)
    await org.loadLocations()
  } catch (e) {
    error.value = describe(e)
  }
}
const initial = computed(() => (editing.value ? { ...editing.value } : { parent_id: parentFor.value, status: 'active' }))
const submit = (v: Record<string, unknown>) => (editing.value ? org.updateLocation(editing.value.id, v) : org.createLocation(v))
</script>

<template>
  <div>
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Locations</h1>
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-plus" class="me-2" @click="add()">New location</v-btn>
      <v-btn variant="text" icon="mdi-refresh" @click="org.loadLocations()" />
    </div>
    <v-alert v-if="error || org.error" type="error" variant="tonal" density="compact" class="mb-3">{{ error || org.error }}</v-alert>
    <v-card>
      <v-card-text>
        <div v-if="org.locationTree.length === 0" class="text-medium-emphasis">No locations yet.</div>
        <TreeNodes :items="org.locationTree as TreeItem[]" @edit="edit" @remove="remove" @add-child="add" />
      </v-card-text>
    </v-card>
    <RecordDialog v-model="dialog" :title="editing ? 'Edit location' : 'New location'" :fields="fields" :initial="initial" :submit="submit" @saved="org.loadLocations()" />
  </div>
</template>
