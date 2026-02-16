package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/go-tangra/go-tangra-asset/internal/data"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type InsurancePolicyService struct {
	assetV1.UnimplementedInsurancePolicyServiceServer

	log        *log.Helper
	policyRepo *data.InsurancePolicyRepo
	assetRepo  *data.AssetRepo
}

func NewInsurancePolicyService(
	ctx *bootstrap.Context,
	policyRepo *data.InsurancePolicyRepo,
	assetRepo *data.AssetRepo,
) *InsurancePolicyService {
	return &InsurancePolicyService{
		log:        ctx.NewLoggerHelper("asset/service/insurance-policy"),
		policyRepo: policyRepo,
		assetRepo:  assetRepo,
	}
}

func (s *InsurancePolicyService) CreateInsurancePolicy(ctx context.Context, req *assetV1.CreateInsurancePolicyRequest) (*assetV1.CreateInsurancePolicyResponse, error) {
	opts := []func(*ent.InsurancePolicyCreate){}

	if req.PolicyNumber != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetPolicyNumber(*req.PolicyNumber) })
	}
	if req.Provider != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetProvider(*req.Provider) })
	}
	if req.CoverageType != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetCoverageType(coverageTypeToString(*req.CoverageType)) })
	}
	if req.PremiumAmount != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetPremiumAmount(*req.PremiumAmount) })
	}
	if req.Deductible != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetDeductible(*req.Deductible) })
	}
	if req.CoverageLimit != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetCoverageLimit(*req.CoverageLimit) })
	}
	if req.ValidFrom != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetValidFrom(req.ValidFrom.AsTime()) })
	}
	if req.ValidTo != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetValidTo(req.ValidTo.AsTime()) })
	}
	if req.Notes != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetNotes(*req.Notes) })
	}
	if req.Status != nil {
		opts = append(opts, func(c *ent.InsurancePolicyCreate) { c.SetStatus(insurancePolicyStatusToString(*req.Status)) })
	}

	entity, err := s.policyRepo.Create(ctx, req.GetTenantId(), req.GetName(), opts...)
	if err != nil {
		return nil, err
	}

	return &assetV1.CreateInsurancePolicyResponse{
		InsurancePolicy: insurancePolicyToProto(entity, 0),
	}, nil
}

func (s *InsurancePolicyService) GetInsurancePolicy(ctx context.Context, req *assetV1.GetInsurancePolicyRequest) (*assetV1.GetInsurancePolicyResponse, error) {
	entity, err := s.policyRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, assetV1.ErrorInsurancePolicyNotFound("insurance policy not found")
	}

	assetCount, err := s.policyRepo.GetAssetCount(ctx, entity.ID)
	if err != nil {
		return nil, err
	}

	return &assetV1.GetInsurancePolicyResponse{
		InsurancePolicy: insurancePolicyToProto(entity, assetCount),
	}, nil
}

func (s *InsurancePolicyService) ListInsurancePolicies(ctx context.Context, req *assetV1.ListInsurancePoliciesRequest) (*assetV1.ListInsurancePoliciesResponse, error) {
	filters := make(map[string]interface{})
	if req.Status != nil && *req.Status != assetV1.InsurancePolicyStatus_INSURANCE_POLICY_STATUS_UNSPECIFIED {
		filters["status"] = insurancePolicyStatusToString(*req.Status)
	}
	if req.CoverageType != nil && *req.CoverageType != assetV1.CoverageType_COVERAGE_TYPE_UNSPECIFIED {
		filters["coverage_type"] = coverageTypeToString(*req.CoverageType)
	}
	if req.Query != nil {
		filters["query"] = *req.Query
	}

	page := int(req.GetPage())
	pageSize := int(req.GetPageSize())
	if req.GetNoPaging() {
		page = 0
		pageSize = 0
	}

	entities, total, err := s.policyRepo.List(ctx, req.GetTenantId(), page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.InsurancePolicy, len(entities))
	for i, e := range entities {
		ac, _ := s.policyRepo.GetAssetCount(ctx, e.ID)
		items[i] = insurancePolicyToProto(e, ac)
	}

	return &assetV1.ListInsurancePoliciesResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *InsurancePolicyService) UpdateInsurancePolicy(ctx context.Context, req *assetV1.UpdateInsurancePolicyRequest) (*assetV1.UpdateInsurancePolicyResponse, error) {
	updates := make(map[string]interface{})

	if req.Data != nil {
		if req.Data.Name != nil {
			updates["name"] = *req.Data.Name
		}
		if req.Data.PolicyNumber != nil {
			updates["policy_number"] = *req.Data.PolicyNumber
		}
		if req.Data.Provider != nil {
			updates["provider"] = *req.Data.Provider
		}
		if req.Data.CoverageType != nil {
			updates["coverage_type"] = coverageTypeToString(*req.Data.CoverageType)
		}
		if req.Data.PremiumAmount != nil {
			updates["premium_amount"] = *req.Data.PremiumAmount
		}
		if req.Data.Deductible != nil {
			updates["deductible"] = *req.Data.Deductible
		}
		if req.Data.CoverageLimit != nil {
			updates["coverage_limit"] = *req.Data.CoverageLimit
		}
		if req.Data.ValidFrom != nil {
			updates["valid_from"] = req.Data.ValidFrom.AsTime()
		}
		if req.Data.ValidTo != nil {
			updates["valid_to"] = req.Data.ValidTo.AsTime()
		}
		if req.Data.Notes != nil {
			updates["notes"] = *req.Data.Notes
		}
		if req.Data.Status != nil {
			updates["status"] = insurancePolicyStatusToString(*req.Data.Status)
		}
	}

	entity, err := s.policyRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	assetCount, _ := s.policyRepo.GetAssetCount(ctx, entity.ID)

	return &assetV1.UpdateInsurancePolicyResponse{
		InsurancePolicy: insurancePolicyToProto(entity, assetCount),
	}, nil
}

func (s *InsurancePolicyService) DeleteInsurancePolicy(ctx context.Context, req *assetV1.DeleteInsurancePolicyRequest) (*emptypb.Empty, error) {
	hasAssets, err := s.policyRepo.HasAssets(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if hasAssets {
		return nil, assetV1.ErrorPolicyHasAssets("cannot delete policy with associated assets")
	}

	err = s.policyRepo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *InsurancePolicyService) ListPolicyAssets(ctx context.Context, req *assetV1.ListPolicyAssetsRequest) (*assetV1.ListPolicyAssetsResponse, error) {
	entities, err := s.policyRepo.ListAssets(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.PolicyAsset, len(entities))
	for i, e := range entities {
		items[i] = policyAssetToProto(e)
	}

	return &assetV1.ListPolicyAssetsResponse{
		Items: items,
	}, nil
}

func (s *InsurancePolicyService) AddAssetToPolicy(ctx context.Context, req *assetV1.AddAssetToPolicyRequest) (*assetV1.AddAssetToPolicyResponse, error) {
	policy, err := s.policyRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, assetV1.ErrorInsurancePolicyNotFound("insurance policy not found")
	}

	a, err := s.assetRepo.GetByID(ctx, req.GetAssetId())
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, assetV1.ErrorAssetNotFound("asset not found")
	}

	var coveredValue *float64
	if req.CoveredValue != nil {
		coveredValue = req.CoveredValue
	}
	notes := ""
	if req.Notes != nil {
		notes = *req.Notes
	}

	var tenantID uint32
	if policy.TenantID != nil {
		tenantID = *policy.TenantID
	}

	entity, err := s.policyRepo.AddAsset(
		ctx,
		tenantID,
		policy.ID,
		a.ID,
		coveredValue,
		notes,
		a.AssetTag,
		a.Name,
		a.ModelName,
	)
	if err != nil {
		return nil, err
	}

	return &assetV1.AddAssetToPolicyResponse{
		PolicyAsset: policyAssetToProto(entity),
	}, nil
}

func (s *InsurancePolicyService) RemoveAssetFromPolicy(ctx context.Context, req *assetV1.RemoveAssetFromPolicyRequest) (*emptypb.Empty, error) {
	err := s.policyRepo.RemoveAsset(ctx, req.GetId(), req.GetAssetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func insurancePolicyToProto(e *ent.InsurancePolicy, assetCount int) *assetV1.InsurancePolicy {
	if e == nil {
		return nil
	}

	result := &assetV1.InsurancePolicy{
		Id:           &e.ID,
		TenantId:     e.TenantID,
		Name:         ptrString(e.Name),
		PolicyNumber: ptrString(e.PolicyNumber),
		Provider:     ptrString(e.Provider),
		CoverageType: coverageTypeFromString(e.CoverageType).Enum(),
		Notes:        ptrString(e.Notes),
		Status:       insurancePolicyStatusFromString(e.Status).Enum(),
		AssetCount:   ptrInt32(int32(assetCount)),
		CreatedBy:    e.CreateBy,
		UpdatedBy:    e.UpdateBy,
	}

	if e.PremiumAmount != nil {
		result.PremiumAmount = e.PremiumAmount
	}
	if e.Deductible != nil {
		result.Deductible = e.Deductible
	}
	if e.CoverageLimit != nil {
		result.CoverageLimit = e.CoverageLimit
	}
	if e.ValidFrom != nil {
		result.ValidFrom = timestamppb.New(*e.ValidFrom)
	}
	if e.ValidTo != nil {
		result.ValidTo = timestamppb.New(*e.ValidTo)
	}
	if e.CreateTime != nil {
		result.CreatedAt = timestamppb.New(*e.CreateTime)
	}
	if e.UpdateTime != nil {
		result.UpdatedAt = timestamppb.New(*e.UpdateTime)
	}

	return result
}

func policyAssetToProto(e *ent.InsurancePolicyAsset) *assetV1.PolicyAsset {
	if e == nil {
		return nil
	}

	result := &assetV1.PolicyAsset{
		PolicyId:  ptrString(e.PolicyID),
		AssetId:   ptrString(e.AssetID),
		AssetTag:  ptrString(e.AssetTag),
		AssetName: ptrString(e.AssetName),
		ModelName: ptrString(e.ModelName),
	}

	if e.CoveredValue != nil {
		result.CoveredValue = e.CoveredValue
	}
	if e.Notes != "" {
		result.Notes = ptrString(e.Notes)
	}

	return result
}

func insurancePolicyStatusToString(s assetV1.InsurancePolicyStatus) string {
	switch s {
	case assetV1.InsurancePolicyStatus_INSURANCE_POLICY_STATUS_ACTIVE:
		return "ACTIVE"
	case assetV1.InsurancePolicyStatus_INSURANCE_POLICY_STATUS_EXPIRED:
		return "EXPIRED"
	case assetV1.InsurancePolicyStatus_INSURANCE_POLICY_STATUS_CANCELLED:
		return "CANCELLED"
	default:
		return "ACTIVE"
	}
}

func insurancePolicyStatusFromString(s string) assetV1.InsurancePolicyStatus {
	switch s {
	case "ACTIVE":
		return assetV1.InsurancePolicyStatus_INSURANCE_POLICY_STATUS_ACTIVE
	case "EXPIRED":
		return assetV1.InsurancePolicyStatus_INSURANCE_POLICY_STATUS_EXPIRED
	case "CANCELLED":
		return assetV1.InsurancePolicyStatus_INSURANCE_POLICY_STATUS_CANCELLED
	default:
		return assetV1.InsurancePolicyStatus_INSURANCE_POLICY_STATUS_UNSPECIFIED
	}
}

func coverageTypeToString(ct assetV1.CoverageType) string {
	switch ct {
	case assetV1.CoverageType_COVERAGE_TYPE_ALL_RISK:
		return "ALL_RISK"
	case assetV1.CoverageType_COVERAGE_TYPE_FIRE_THEFT:
		return "FIRE_THEFT"
	case assetV1.CoverageType_COVERAGE_TYPE_LIABILITY:
		return "LIABILITY"
	case assetV1.CoverageType_COVERAGE_TYPE_EQUIPMENT_BREAKDOWN:
		return "EQUIPMENT_BREAKDOWN"
	case assetV1.CoverageType_COVERAGE_TYPE_CYBER:
		return "CYBER"
	default:
		return "ALL_RISK"
	}
}

func coverageTypeFromString(s string) assetV1.CoverageType {
	switch s {
	case "ALL_RISK":
		return assetV1.CoverageType_COVERAGE_TYPE_ALL_RISK
	case "FIRE_THEFT":
		return assetV1.CoverageType_COVERAGE_TYPE_FIRE_THEFT
	case "LIABILITY":
		return assetV1.CoverageType_COVERAGE_TYPE_LIABILITY
	case "EQUIPMENT_BREAKDOWN":
		return assetV1.CoverageType_COVERAGE_TYPE_EQUIPMENT_BREAKDOWN
	case "CYBER":
		return assetV1.CoverageType_COVERAGE_TYPE_CYBER
	default:
		return assetV1.CoverageType_COVERAGE_TYPE_UNSPECIFIED
	}
}
