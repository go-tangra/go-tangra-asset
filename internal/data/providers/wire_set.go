//go:build wireinject
// +build wireinject

//go:generate go run github.com/google/wire/cmd/wire

package providers

import (
	"github.com/google/wire"

	"github.com/go-tangra/go-tangra-asset/internal/data"
)

var ProviderSet = wire.NewSet(
	data.NewRedisClient,
	data.NewEntClient,
	data.NewStorageClient,
	data.NewLdapClient,
	data.NewInventoryClient,
	data.NewSupplierRepo,
	data.NewEmployeeRepo,
	data.NewLocationRepo,
	data.NewCategoryRepo,
	data.NewAssetRepo,
	data.NewAssignmentRepo,
	data.NewDocumentRepo,
	data.NewConsumableRepo,
	data.NewLicenseRepo,
	data.NewInsurancePolicyRepo,
	data.NewAuditLogRepo,
	data.NewStatisticsRepo,
)
