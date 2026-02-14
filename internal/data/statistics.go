package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/asset"
)

// StatisticsRepo provides methods for collecting asset statistics
type StatisticsRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewStatisticsRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *StatisticsRepo {
	return &StatisticsRepo{
		entClient: entClient,
		log:       ctx.NewLoggerHelper("asset/statistics/repo"),
	}
}

func (r *StatisticsRepo) GetAssetCount(ctx context.Context) (int64, error) {
	count, err := r.entClient.Client().Asset.Query().Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *StatisticsRepo) GetAssetCountByStatus(ctx context.Context, status int32) (int64, error) {
	count, err := r.entClient.Client().Asset.Query().
		Where(asset.StatusEQ(status)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *StatisticsRepo) GetConsumableCount(ctx context.Context) (int64, error) {
	count, err := r.entClient.Client().Consumable.Query().Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *StatisticsRepo) GetSupplierCount(ctx context.Context) (int64, error) {
	count, err := r.entClient.Client().Supplier.Query().Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *StatisticsRepo) GetEmployeeCount(ctx context.Context) (int64, error) {
	count, err := r.entClient.Client().Employee.Query().Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *StatisticsRepo) GetCategoryCount(ctx context.Context) (int64, error) {
	count, err := r.entClient.Client().Category.Query().Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *StatisticsRepo) GetLocationCount(ctx context.Context) (int64, error) {
	count, err := r.entClient.Client().Location.Query().Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *StatisticsRepo) GetTotalCost(ctx context.Context) (float64, error) {
	assets, err := r.entClient.Client().Asset.Query().All(ctx)
	if err != nil {
		return 0, err
	}

	var total float64
	for _, a := range assets {
		if a.PurchaseCost != nil {
			total += *a.PurchaseCost
		}
	}
	return total, nil
}
