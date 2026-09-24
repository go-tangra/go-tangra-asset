import { defineStore } from 'pinia'
import { ref } from 'vue'

// A single shared EventSource relays the module's live events (asset.assigned,
// asset.unassigned, asset.warranty.expiring, license.expiring,
// insurance.expiring, consumable.low_stock) through the gateway. The stream is
// reference-counted so several views share one connection.
export type Listener = (type: string, data: unknown) => void

const EVENTS = ['asset.assigned', 'asset.unassigned', 'asset.warranty.expiring', 'license.expiring', 'insurance.expiring', 'consumable.low_stock']

export const useLive = defineStore('asset-live', () => {
  const connected = ref(false)
  const recent = ref<{ type: string; at: string; data: unknown }[]>([])
  let source: EventSource | null = null
  let refs = 0
  const listeners = new Set<Listener>()

  function handle(type: string, raw: string): void {
    let data: unknown = {}
    try {
      data = JSON.parse(raw)
    } catch {
      /* non-JSON payloads are ignored */
    }
    recent.value = [{ type, at: new Date().toISOString(), data }, ...recent.value].slice(0, 20)
    for (const l of listeners) l(type, data)
  }

  function open(): void {
    if (source) return
    source = new EventSource('/api/asset/v1/stream', { withCredentials: true })
    source.onopen = () => (connected.value = true)
    source.onerror = () => (connected.value = false)
    for (const t of EVENTS) source.addEventListener(t, (e) => handle(t, (e as MessageEvent).data))
  }

  /** Opens the stream (first caller) and returns a release function. */
  function connect(): () => void {
    refs += 1
    open()
    return () => {
      refs -= 1
      if (refs <= 0) close()
    }
  }

  function close(): void {
    refs = 0
    source?.close()
    source = null
    connected.value = false
  }

  function on(l: Listener): () => void {
    listeners.add(l)
    return () => listeners.delete(l)
  }

  function _emit(type: string, raw: string): void {
    handle(type, raw)
  }

  return { connected, recent, connect, close, on, _emit }
})
