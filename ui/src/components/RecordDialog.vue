<script setup lang="ts">
// A schema-driven create/edit dialog shared by every record type. Fields are
// declared by the view; the dialog owns validation of "required" fields and
// surfaces the API refusal reason inline.
import { ref, watch } from 'vue'
import { describe } from '@/api/client'

export interface FieldOption {
  title: string
  value: string
}
export interface Field {
  key: string
  label: string
  type?: 'text' | 'number' | 'date' | 'select' | 'textarea'
  options?: FieldOption[]
  required?: boolean
  hint?: string
  cols?: number
}

const props = defineProps<{
  modelValue: boolean
  title: string
  fields: Field[]
  initial?: Record<string, unknown> | undefined
  submit: (values: Record<string, unknown>) => Promise<unknown>
}>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'saved', v: unknown): void }>()

const values = ref<Record<string, unknown>>({})
const error = ref('')
const busy = ref(false)

watch(
  () => [props.modelValue, props.initial] as const,
  ([open]) => {
    if (!open) return
    error.value = ''
    const v: Record<string, unknown> = {}
    for (const f of props.fields) {
      const raw = props.initial?.[f.key]
      if (f.type === 'date' && typeof raw === 'string') v[f.key] = raw.slice(0, 10)
      else v[f.key] = raw ?? (f.type === 'number' ? 0 : '')
    }
    values.value = v
  },
  { immediate: true },
)

function close(): void {
  emit('update:modelValue', false)
}

async function save(): Promise<void> {
  for (const f of props.fields) {
    if (f.required && !String(values.value[f.key] ?? '').trim()) {
      error.value = f.label + ' is required.'
      return
    }
  }
  const out: Record<string, unknown> = {}
  for (const f of props.fields) {
    const v = values.value[f.key]
    if (f.type === 'number') out[f.key] = Number(v ?? 0)
    else if (f.type === 'date') out[f.key] = v ? new Date(String(v)).toISOString() : null
    else out[f.key] = v ?? ''
  }
  busy.value = true
  error.value = ''
  try {
    const res = await props.submit(out)
    emit('saved', res)
    close()
  } catch (e) {
    error.value = describe(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-dialog :model-value="modelValue" max-width="720" @update:model-value="close">
    <v-card>
      <v-card-title>{{ title }}</v-card-title>
      <v-card-text>
        <v-alert v-if="error" type="error" variant="tonal" density="compact" class="mb-3">{{ error }}</v-alert>
        <v-row dense>
          <v-col v-for="f in fields" :key="f.key" cols="12" :md="f.cols ?? 6">
            <v-select v-if="f.type === 'select'" v-model="values[f.key]" :label="f.label" :items="f.options ?? []" :hint="f.hint" persistent-hint clearable density="comfortable" />
            <v-textarea v-else-if="f.type === 'textarea'" v-model="values[f.key]" :label="f.label" rows="2" density="comfortable" />
            <v-text-field v-else v-model="values[f.key]" :label="f.label" :type="f.type === 'number' ? 'number' : f.type === 'date' ? 'date' : 'text'" :hint="f.hint" persistent-hint density="comfortable" />
          </v-col>
        </v-row>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="close">Cancel</v-btn>
        <v-btn color="primary" :loading="busy" @click="save">Save</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
