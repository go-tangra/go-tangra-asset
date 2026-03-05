import './styles/tailwind.css';
import type { TangraModule } from './sdk';
import routes from './routes';
import { useAssetSupplierStore } from './stores/asset-supplier.state';
import { useAssetUserStore } from './stores/asset-user.state';
import { useAssetLocationStore } from './stores/asset-location.state';
import { useAssetCategoryStore } from './stores/asset-category.state';
import { useAssetAssetStore } from './stores/asset-asset.state';
import { useAssetConsumableStore } from './stores/asset-consumable.state';
import { useAssetLicenseStore } from './stores/asset-license.state';
import { useAssetInsuranceStore } from './stores/asset-insurance.state';
import { useAssetSystemStore } from './stores/asset-system.state';
import enUS from './locales/en-US.json';

const assetModule: TangraModule = {
  id: 'asset',
  version: '1.0.0',
  routes,
  stores: {
    'asset-supplier': useAssetSupplierStore,
    'asset-user': useAssetUserStore,
    'asset-location': useAssetLocationStore,
    'asset-category': useAssetCategoryStore,
    'asset-asset': useAssetAssetStore,
    'asset-consumable': useAssetConsumableStore,
    'asset-license': useAssetLicenseStore,
    'asset-insurance': useAssetInsuranceStore,
    'asset-system': useAssetSystemStore,
  },
  locales: {
    'en-US': enUS,
  },
};

export default assetModule;
