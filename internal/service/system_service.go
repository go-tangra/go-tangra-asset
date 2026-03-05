package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/go-tangra/go-tangra-asset/internal/data"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

var version = "1.0.0"

type SystemService struct {
	assetV1.UnimplementedSystemServiceServer

	log       *log.Helper
	statsRepo *data.StatisticsRepo
}

func NewSystemService(ctx *bootstrap.Context, statsRepo *data.StatisticsRepo) *SystemService {
	return &SystemService{
		log:       ctx.NewLoggerHelper("asset/service/system"),
		statsRepo: statsRepo,
	}
}

func (s *SystemService) HealthCheck(ctx context.Context, req *assetV1.HealthCheckRequest) (*assetV1.HealthCheckResponse, error) {
	return &assetV1.HealthCheckResponse{
		Status:    "healthy",
		Version:   version,
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}

func (s *SystemService) GetDashboardStats(ctx context.Context, req *assetV1.GetDashboardStatsRequest) (*assetV1.GetDashboardStatsResponse, error) {
	response := &assetV1.GetDashboardStatsResponse{}

	if count, err := s.statsRepo.GetAssetCount(ctx); err == nil {
		response.TotalAssets = count
	}
	if count, err := s.statsRepo.GetAssetCountByStatus(ctx, 1); err == nil {
		response.AssetsDeployable = count
	}
	if count, err := s.statsRepo.GetAssetCountByStatus(ctx, 2); err == nil {
		response.AssetsAssigned = count
	}
	if count, err := s.statsRepo.GetAssetCountByStatus(ctx, 3); err == nil {
		response.AssetsBroken = count
	}
	if count, err := s.statsRepo.GetAssetCountByStatus(ctx, 4); err == nil {
		response.AssetsArchived = count
	}
	if count, err := s.statsRepo.GetConsumableCount(ctx); err == nil {
		response.TotalConsumables = count
	}
	if count, err := s.statsRepo.GetSupplierCount(ctx); err == nil {
		response.TotalSuppliers = count
	}
	if count, err := s.statsRepo.GetCategoryCount(ctx); err == nil {
		response.TotalCategories = count
	}
	if count, err := s.statsRepo.GetLocationCount(ctx); err == nil {
		response.TotalLocations = count
	}
	if cost, err := s.statsRepo.GetTotalCost(ctx); err == nil {
		response.TotalCost = cost
	}

	return response, nil
}
