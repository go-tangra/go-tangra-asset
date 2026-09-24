import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { axe } from 'vitest-axe'
import Suppliers from '@/views/suppliers/index.vue'
import Assets from '@/views/assets/index.vue'
import Categories from '@/views/categories/index.vue'
import Dashboard from '@/views/dashboard/index.vue'
import Detail from '@/views/assets/detail.vue'

type Handler = (url: string, init: RequestInit) => unknown
function fetchMock(handler: Handler) {
  const calls: { url: string; init: RequestInit }[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
    calls.push({ url, init })
    const body = handler(url, init)
    if (body === 204) return new Response(null, { status: 204 })
    return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }))
  return calls
}
class FakeSource {
  static instances: FakeSource[] = []
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  listeners = new Map<string, (e: MessageEvent) => void>()
  constructor() { FakeSource.instances.push(this) }
  addEventListener(t: string, fn: (e: MessageEvent) => void) { this.listeners.set(t, fn) }
  close() {}
  emit(t: string, data: unknown) { this.listeners.get(t)?.(new MessageEvent(t, { data: JSON.stringify(data) })) }
}
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div/>' } }, { path: '/asset', name: 'asset-assets', component: { template: '<div/>' } }, { path: '/asset/item/:id', name: 'asset-detail', component: { template: '<div/>' } }] })
const global = { plugins: [router] }
const supplier = { id: 's1', name: 'ACME', code: 'AC', city: 'Sofia', country: 'BG', status: 'active' }
const asset = { id: 'a1', tenant_id: 't', asset_tag: 'AST-1', name: 'Laptop', status: 'deployable', has_photo: false, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z', book_value: 1200.5, category_id: 'c1' }

describe('asset views on the kit', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.cookie = '__Host-csrf=tok; Secure; Path=/'
    vi.stubGlobal('EventSource', FakeSource)
    FakeSource.instances = []
    ;(globalThis as unknown as { __vw: number }).__vw = 1280
  })

  it('suppliers: table, create through the schema dialog (payload is z.output), no inline styles, axe clean', async () => {
    const calls = fetchMock((url, init) => (init.method === 'POST' ? { ...supplier, id: 's2' } : { items: [supplier] }))
    const w = mount(Suppliers, { global, attachTo: document.body })
    await flushPromises()
    expect(w.find('table').exists()).toBe(true)
    expect(w.text()).toContain('ACME')
    expect(w.find('[style]').exists()).toBe(false)
    await w.find('button.btn-primary').trigger('click') // New supplier
    await flushPromises()
    const dialog = document.body.querySelector('[role=dialog]')!
    expect(dialog).toBeTruthy()
    const name = dialog.querySelector<HTMLInputElement>('input[data-field=name]')!
    name.value = '  New Co '
    name.dispatchEvent(new Event('input'))
    const email = dialog.querySelector<HTMLInputElement>('input[data-field=email]')!
    email.value = 'Sales@New.TEST'
    email.dispatchEvent(new Event('input'))
    dialog.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()
    const post = calls.find((c) => c.init.method === 'POST')!
    expect(post.url).toBe('/api/asset/v1/suppliers')
    expect((post.init.headers as Record<string, string>)['X-CSRF-Token']).toBe('tok')
    expect(JSON.parse(String(post.init.body))).toEqual({ name: 'New Co', email: 'sales@new.test', status: 'active' })
    expect((await axe(w.element as HTMLElement, { rules: { 'color-contrast': { enabled: false }, region: { enabled: false } } })).violations).toEqual([])
    w.unmount()
  })

  it('suppliers: invalid input blocks the request and focuses the first invalid field', async () => {
    const calls = fetchMock(() => ({ items: [] }))
    const w = mount(Suppliers, { global, attachTo: document.body })
    await flushPromises()
    await w.find('button.btn-primary').trigger('click')
    await flushPromises()
    const dialog = document.body.querySelector('[role=dialog]')!
    dialog.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()
    expect(calls.some((c) => c.init.method === 'POST')).toBe(false)
    expect(dialog.querySelector('[role=alert]')?.textContent).toContain('required')
    expect(document.activeElement?.getAttribute('data-field')).toBe('name')
    w.unmount()
  })

  it('assets: list rows, stacked cards below md, a stream event reloads the list', async () => {
    const calls = fetchMock((url) => (url.includes('/assets') ? { items: [asset] } : { items: [] }))
    const w = mount(Assets, { global })
    await flushPromises()
    expect(w.find('table').exists()).toBe(true)
    expect(w.text()).toContain('AST-1')
    expect(w.text()).toContain('1,200.5')
    const before = calls.filter((c) => c.url.startsWith('/api/asset/v1/assets')).length
    FakeSource.instances[0]!.emit('asset.assigned', { id: 'a1' })
    await flushPromises()
    expect(calls.filter((c) => c.url.startsWith('/api/asset/v1/assets')).length).toBe(before + 1)
    w.unmount()
    ;(globalThis as unknown as { __vw: number }).__vw = 360
    const m = mount(Assets, { global })
    await flushPromises()
    expect(m.find('table').exists()).toBe(false)
    expect(m.findAll('li.card').length).toBe(1)
    m.unmount()
  })

  it('categories: tree with a selection panel gating delete on counts', async () => {
    fetchMock((url) => (url.endsWith('/categories/tree') ? { items: [{ id: 'c1', name: 'Hardware', asset_count: 2, child_count: 1, children: [{ id: 'c2', name: 'Laptops', asset_count: 0, child_count: 0 }] }] } : { items: [{ id: 'c1', name: 'Hardware', asset_count: 2, child_count: 1 }, { id: 'c2', name: 'Laptops', asset_count: 0, child_count: 0 }] }))
    const w = mount(Categories, { global })
    await flushPromises()
    expect(w.find('[role=tree]').exists()).toBe(true)
    const items = w.findAll('[role=treeitem]')
    expect(items.length).toBe(2)
    await items[0]!.trigger('click')
    expect(w.text()).toContain('2 assets')
    const del = w.findAll('button').find((b) => b.text().includes('Delete'))!
    expect(del.attributes('disabled')).toBeDefined()
    await items[1]!.trigger('click')
    expect(w.findAll('button').find((b) => b.text().includes('Delete'))!.attributes('disabled')).toBeUndefined()
    w.unmount()
  })

  it('dashboard: stat tiles and status bars from the snapshot', async () => {
    fetchMock(() => ({ total_assets: 12, assigned_assets: 3, assets_by_status: { deployable: 9, assigned: 3 }, total_cost: 1000, expiring_soon: 1 }))
    const w = mount(Dashboard, { global })
    await flushPromises()
    expect(w.findAll('.stat-tile').length).toBe(10)
    expect(w.text()).toContain('3 assigned')
    expect(w.findAll('progress').length).toBe(2)
    w.unmount()
  })

  it('detail: key/value cards, history table, documents list, assign dialog validates', async () => {
    await router.push('/asset/item/a1')
    fetchMock((url) => (url.endsWith('/assets/a1') ? asset : url.endsWith('/assignments') ? { items: [{ id: 'h1', asset_id: 'a1', action: 'assigned', assigned_at: '2026-01-02T00:00:00Z', user_name: 'Ann' }] } : url.endsWith('/users') ? { items: [{ id: 'u1', display_name: 'Ann' }] } : { items: [] }))
    const w = mount(Detail, { global, attachTo: document.body })
    await flushPromises()
    expect(w.find('h1').text()).toBe('AST-1')
    expect(w.findAll('dl').length).toBeGreaterThanOrEqual(2)
    expect(w.text()).toContain('Ann')
    expect(w.text()).toContain('Documents')
    await w.findAll('button').find((b) => b.text().includes('Assign'))!.trigger('click')
    await flushPromises()
    const dialog = document.body.querySelector('[role=dialog]')!
    const assign = Array.from(dialog.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Assign')!
    assign.click()
    await flushPromises()
    expect(dialog.querySelector('[role=alert]')?.textContent).toContain('required')
    w.unmount()
  })
})
