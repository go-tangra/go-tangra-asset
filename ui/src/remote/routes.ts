import type { RouteRecordRaw } from 'vue-router'
import '@/main.css'

// Routes mounted by the platform shell under their own error boundary.
const m = { module: 'asset' }
export const routes: RouteRecordRaw[] = [
  { path: '/asset', name: 'asset-assets', component: () => import('@/views/assets/index.vue'), meta: m },
  { path: '/asset/item/:id', name: 'asset-detail', component: () => import('@/views/assets/detail.vue'), meta: m },
  { path: '/asset/categories', name: 'asset-categories', component: () => import('@/views/categories/index.vue'), meta: m },
  { path: '/asset/suppliers', name: 'asset-suppliers', component: () => import('@/views/suppliers/index.vue'), meta: m },
  { path: '/asset/locations', name: 'asset-locations', component: () => import('@/views/locations/index.vue'), meta: m },
  { path: '/asset/consumables', name: 'asset-consumables', component: () => import('@/views/consumables/index.vue'), meta: m },
  { path: '/asset/licenses', name: 'asset-licenses', component: () => import('@/views/licenses/index.vue'), meta: m },
  { path: '/asset/insurance', name: 'asset-insurance', component: () => import('@/views/insurance/index.vue'), meta: m },
  { path: '/asset/inventory-sync', name: 'asset-inventory-sync', component: () => import('@/views/inventory-sync/index.vue'), meta: m },
  { path: '/asset/dashboard', name: 'asset-dashboard', component: () => import('@/views/dashboard/index.vue'), meta: m },
]
export default routes
