// Server paging and sorting of every asset table (go-tangra
// specs/032-server-side-tables): each table asks for one page with the
// server's sort fields, shows the total, keeps page / size / sort in the URL,
// adopts the server-clamped page, returns to page 1 on a filter change and
// reloads (debounced) on asset.* live events.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import Assets from '@/views/assets/index.vue'
import Detail from '@/views/assets/detail.vue'
import Suppliers from '@/views/suppliers/index.vue'
import Consumables from '@/views/consumables/index.vue'
import Licenses from '@/views/licenses/index.vue'
import Insurance from '@/views/insurance/index.vue'
import { pagedList } from '@/stores/paged'
import { coalesce } from '@/stores/live'

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
const routes = [
  { path: '/', component: { template: '<div/>' } },
  { path: '/asset', name: 'asset-assets', component: { template: '<div/>' } },
  { path: '/asset/item/:id', name: 'asset-detail', component: { template: '<div/>' } },
]
const newRouter = () => createRouter({ history: createMemoryHistory(), routes })
const params = (url: string) => new URL(url, 'https://x').searchParams
const header = (w: ReturnType<typeof mount>, label: string) => w.findAll('th button').find((b) => b.text().startsWith(label))
const asset = { id: 'a1', tenant_id: 't', asset_tag: 'AST-1', name: 'Laptop', status: 'deployable', has_photo: false, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z', book_value: 10 }

/** A server of `total` rows per list that echoes the request and clamps the page. */
function server(total = 130, row: (path: string, page: number) => Record<string, unknown> = (_p, page) => ({ id: 'r' + page, name: 'Row ' + page })) {
  return (url: string) => {
    const path = new URL(url, 'https://x').pathname
    const q = params(url)
    if (!q.has('page')) return path.endsWith('/assets/a1') ? asset : { items: [] }
    const size = Number(q.get('page_size'))
    const page = Math.min(Number(q.get('page')), Math.max(1, Math.ceil(total / size)))
    return { items: [row(path, page)], total, page, page_size: size, sort: q.get('sort'), order: q.get('order') }
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  document.cookie = '__Host-csrf=tok; Secure; Path=/'
  vi.stubGlobal('EventSource', FakeSource)
  FakeSource.instances = []
  ;(globalThis as unknown as { __vw: number }).__vw = 1280
})

describe('assets table', () => {
  it('pages with the total, sorts the whole list on the server, filters return to page 1', async () => {
    const calls = fetchMock(server(130, (_p, page) => ({ ...asset, id: 'a' + page, asset_tag: 'AST-' + page })))
    const w = mount(Assets, { global: { plugins: [newRouter()] }, attachTo: document.body })
    await flushPromises()
    const lists = () => calls.filter((c) => c.url.startsWith('/api/asset/v1/assets?'))
    const last = () => params(lists().at(-1)!.url)
    expect(lists()[0]!.url).toBe('/api/asset/v1/assets?page=1&page_size=25&sort=asset_tag&order=asc')
    expect(w.text()).toContain('Showing 1–25 of 130')
    await w.find('[aria-label="Page 3"]').trigger('click')
    await flushPromises()
    expect(last().get('page')).toBe('3')
    // Sortable columns are exactly the server's sort fields.
    const sortable = w.findAll('th button').map((b) => b.text())
    for (const label of ['Tag', 'Name', 'Category', 'Status', 'Location', 'Purchased']) expect(sortable.some((t) => t.startsWith(label)), label).toBe(true)
    expect(sortable.some((t) => t.startsWith('Book value'))).toBe(false)
    await header(w, 'Category')!.trigger('click')
    await flushPromises()
    expect([last().get('sort'), last().get('order'), last().get('page')]).toEqual(['category', 'asc', '1'])
    await header(w, 'Purchased')!.trigger('click')
    await flushPromises()
    expect([last().get('sort'), last().get('order')]).toEqual(['purchase_date', 'desc'])
    // A filter change returns to page 1 and keeps the sort.
    await w.find('[aria-label="Page 2"]').trigger('click')
    await flushPromises()
    expect(last().get('page')).toBe('2')
    await w.find('#asset-status').setValue('broken')
    await flushPromises()
    expect([last().get('status'), last().get('page'), last().get('sort')]).toEqual(['broken', '1', 'purchase_date'])
    expect(last().has('cursor') || last().has('limit')).toBe(false)
    w.unmount()
  })

  it('a burst of asset.* events reloads the current page once; other events do not', async () => {
    vi.useFakeTimers()
    try {
      const calls = fetchMock(server(3))
      const w = mount(Assets, { global: { plugins: [newRouter()] } })
      await flushPromises()
      const lists = () => calls.filter((c) => c.url.startsWith('/api/asset/v1/assets?')).length
      const before = lists()
      const src = FakeSource.instances[0]!
      src.emit('asset.assigned', { asset_id: 'a1' })
      src.emit('asset.unassigned', { asset_id: 'a2' })
      src.emit('asset.warranty.expiring', { asset_id: 'a3' })
      src.emit('license.expiring', { license_id: 'l1' })
      await vi.advanceTimersByTimeAsync(500)
      await flushPromises()
      expect(lists()).toBe(before + 1)
      src.emit('consumable.low_stock', { consumable_id: 'c1' })
      await vi.advanceTimersByTimeAsync(500)
      expect(lists()).toBe(before + 1)
      w.unmount()
    } finally {
      vi.useRealTimers()
    }
  })

  it('page, size and sort live in the URL; the server-clamped page is adopted; unknown sorts fall back', async () => {
    const r = newRouter()
    await r.push('/asset?assets.page=9&assets.size=10&assets.sort=warranty_end&assets.order=desc')
    await r.isReady()
    const calls = fetchMock(server(31))
    const w = mount(Assets, { global: { plugins: [r] } })
    await flushPromises()
    const first = params(calls.find((c) => c.url.startsWith('/api/asset/v1/assets?'))!.url)
    expect([first.get('page'), first.get('page_size'), first.get('sort'), first.get('order')]).toEqual(['9', '10', 'warranty_end', 'desc'])
    expect(r.currentRoute.value.query['assets.page']).toBe('4') // server clamped 9 → 4
    w.unmount()
    await r.push('/asset?assets.sort=serial')
    const calls2 = fetchMock(server(31))
    const w2 = mount(Assets, { global: { plugins: [r] } })
    await flushPromises()
    const q2 = params(calls2.find((c) => c.url.startsWith('/api/asset/v1/assets?'))!.url)
    expect([q2.get('page'), q2.get('sort'), q2.get('order')]).toEqual(['1', 'asset_tag', 'asc'])
    w2.unmount()
  })

  it('selects load the suppliers by name up to the largest page', async () => {
    const calls = fetchMock(server(3))
    const w = mount(Assets, { global: { plugins: [newRouter()] } })
    await flushPromises()
    const q = params(calls.find((c) => c.url.startsWith('/api/asset/v1/suppliers?'))!.url)
    expect([q.get('page'), q.get('page_size'), q.get('sort'), q.get('order')]).toEqual(['1', '200', 'name', 'asc'])
    w.unmount()
  })
})

describe('simple tables', () => {
  const cases = [
    { name: 'suppliers', view: Suppliers, path: '/api/asset/v1/suppliers?', first: 'page=1&page_size=25&sort=name&order=asc', search: '#supplier-search', sorts: { Name: ['name', 'desc'] } },
    { name: 'consumables', view: Consumables, path: '/api/asset/v1/consumables?', first: 'page=1&page_size=25&sort=name&order=asc', search: '#consumable-search', sorts: { Stock: ['amount', 'asc'] } },
    { name: 'licenses', view: Licenses, path: '/api/asset/v1/licenses?', first: 'page=1&page_size=25&sort=name&order=asc', search: '#license-search', sorts: { 'Valid to': ['valid_to', 'asc'] } },
    { name: 'insurance', view: Insurance, path: '/api/asset/v1/insurance-policies?', first: 'page=1&page_size=25&sort=name&order=asc', search: '#policy-search', sorts: { 'Valid to': ['valid_to', 'asc'] } },
  ] as const
  for (const c of cases) {
    it(`${c.name}: first page, total, header sort, search back to page 1`, async () => {
      const calls = fetchMock(server(60))
      const w = mount(c.view, { global: { plugins: [newRouter()] }, attachTo: document.body })
      await flushPromises()
      const lists = () => calls.filter((x) => x.url.startsWith(c.path))
      const last = () => params(lists().at(-1)!.url)
      expect(lists()[0]!.url).toBe(c.path + c.first)
      expect(w.text()).toContain('Showing 1–25 of 60')
      for (const [label, [sort, order]] of Object.entries(c.sorts)) {
        await header(w, label)!.trigger('click')
        await flushPromises()
        expect([last().get('sort'), last().get('order')]).toEqual([sort, order])
      }
      await w.find('[aria-label="Page 2"]').trigger('click')
      await flushPromises()
      expect(last().get('page')).toBe('2')
      const input = w.find<HTMLInputElement>(c.search)
      await input.setValue('acme')
      await input.trigger('keyup', { key: 'Enter' })
      await flushPromises()
      expect([last().get('query'), last().get('page')]).toEqual(['acme', '1'])
      w.unmount()
    })
  }

  it('insurance: the covered assets of the selected policy page and sort on the server', async () => {
    const calls = fetchMock(server(60, (path, page) => (path.endsWith('/assets') ? { id: 'pa' + page, asset_id: 'a' + page, asset_tag: 'AST-' + page, asset_name: 'Covered ' + page } : { id: 'p' + page, name: 'Policy ' + page, status: 'active' })))
    const w = mount(Insurance, { global: { plugins: [newRouter()] }, attachTo: document.body })
    await flushPromises()
    await w.find('tbody tr').trigger('click')
    await flushPromises()
    const covered = () => calls.filter((x) => x.url.startsWith('/api/asset/v1/insurance-policies/p1/assets?'))
    expect(covered()[0]!.url).toBe('/api/asset/v1/insurance-policies/p1/assets?page=1&page_size=25&sort=asset_tag&order=asc')
    expect(w.text()).toContain('Covered 1')
    const tables = w.findAll('table')
    const coveredHeader = tables.at(-1)!.findAll('th button').find((b) => b.text().startsWith('Name'))!
    await coveredHeader.trigger('click')
    await flushPromises()
    expect(params(covered().at(-1)!.url).get('sort')).toBe('name')
    // The asset picker loads the first assets by tag, not the assets table.
    const picker = params(calls.find((x) => x.url.startsWith('/api/asset/v1/assets?'))!.url)
    expect([picker.get('page'), picker.get('sort')]).toEqual(['1', 'asset_tag'])
    w.unmount()
  })
})

describe('asset detail assignments', () => {
  it('pages newest first and sorts on the server', async () => {
    const r = newRouter()
    await r.push('/asset/item/a1')
    const calls = fetchMock(server(40, () => ({ id: 'h1', asset_id: 'a1', action: 'assigned', assigned_at: '2026-01-02T00:00:00Z', user_name: 'Ann' })))
    const w = mount(Detail, { global: { plugins: [r] }, attachTo: document.body })
    await flushPromises()
    const hist = () => calls.filter((x) => x.url.startsWith('/api/asset/v1/assets/a1/assignments?'))
    expect(hist()[0]!.url).toBe('/api/asset/v1/assets/a1/assignments?page=1&page_size=25&sort=assigned_at&order=desc')
    expect(w.text()).toContain('Showing 1–25 of 40')
    await header(w, 'Returned')!.trigger('click')
    await flushPromises()
    expect([params(hist().at(-1)!.url).get('sort'), params(hist().at(-1)!.url).get('order')]).toEqual(['returned_at', 'desc'])
    w.unmount()
  })
})

describe('pagedList', () => {
  it('ignores superseded responses and does not send blank filters', async () => {
    const resolvers: ((v: Response) => void)[] = []
    const urls: string[] = []
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      urls.push(url)
      return new Promise<Response>((res) => resolvers.push(res))
    }))
    const list = pagedList<{ id: string }, { query?: string | undefined }>('assets', { page: 1, page_size: 25, sort: 'name', order: 'asc' })
    const first = list.list({ query: '' })
    const second = list.list({ query: 'x' })
    const json = (b: unknown) => new Response(JSON.stringify(b), { status: 200, headers: { 'Content-Type': 'application/json' } })
    resolvers[1]!(json({ items: [{ id: 'new' }], total: 1, page: 1 }))
    resolvers[0]!(json({ items: [{ id: 'old' }], total: 9, page: 1 }))
    expect(await second).not.toBeNull()
    expect(await first).toBeNull()
    expect(list.items.value.map((i) => i.id)).toEqual(['new'])
    expect(list.total.value).toBe(1)
    expect(params(urls[0]!).has('query')).toBe(false)
    expect(params(urls[1]!).get('query')).toBe('x')
  })

  it('coalesce runs once per burst and can be cancelled', () => {
    vi.useFakeTimers()
    try {
      const fn = vi.fn()
      const c = coalesce(fn, 100)
      c.trigger()
      c.trigger()
      vi.advanceTimersByTime(100)
      expect(fn).toHaveBeenCalledTimes(1)
      c.trigger()
      c.cancel()
      vi.advanceTimersByTime(100)
      expect(fn).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })
})
