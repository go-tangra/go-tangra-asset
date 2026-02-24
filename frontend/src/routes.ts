import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    path: '/asset',
    name: 'Asset',
    component: () => import('shell/vben/layouts').then((m) => m.BasicLayout),
    redirect: '/asset/asset',
    meta: {
      order: 2030,
      icon: 'lucide:package',
      title: 'asset.menu.moduleName',
      keepAlive: true,
      authority: ['platform:admin', 'tenant:manager'],
    },
    children: [
      {
        path: 'asset',
        name: 'AssetAssets',
        meta: {
          icon: 'lucide:monitor',
          title: 'asset.menu.assets',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/asset/index.vue'),
      },
      {
        path: 'consumable',
        name: 'AssetConsumables',
        meta: {
          icon: 'lucide:boxes',
          title: 'asset.menu.consumables',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/consumable/index.vue'),
      },
      {
        path: 'license',
        name: 'AssetLicenses',
        meta: {
          icon: 'lucide:scroll-text',
          title: 'asset.menu.licenses',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/license/index.vue'),
      },
      {
        path: 'insurance',
        name: 'AssetInsurance',
        meta: {
          icon: 'lucide:shield-check',
          title: 'asset.menu.insurance',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/insurance/index.vue'),
      },
      {
        path: 'category',
        name: 'AssetCategories',
        meta: {
          icon: 'lucide:folder-tree',
          title: 'asset.menu.categories',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/category/index.vue'),
      },
      {
        path: 'supplier',
        name: 'AssetSuppliers',
        meta: {
          icon: 'lucide:truck',
          title: 'asset.menu.suppliers',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/supplier/index.vue'),
      },
      {
        path: 'employee',
        name: 'AssetEmployees',
        meta: {
          icon: 'lucide:users',
          title: 'asset.menu.employees',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/employee/index.vue'),
      },
      {
        path: 'location',
        name: 'AssetLocations',
        meta: {
          icon: 'lucide:map-pin',
          title: 'asset.menu.locations',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/location/index.vue'),
      },
    ],
  },
];

export default routes;
