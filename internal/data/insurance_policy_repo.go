package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/insurancepolicy"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/insurancepolicyasset"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type InsurancePolicyRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewInsurancePolicyRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *InsurancePolicyRepo {
	return &InsurancePolicyRepo{
		log:       ctx.NewLoggerHelper("asset/insurance-policy/repo"),
		entClient: entClient,
	}
}

func (r *InsurancePolicyRepo) Create(ctx context.Context, tenantID uint32, name string, opts ...func(*ent.InsurancePolicyCreate)) (*ent.InsurancePolicy, error) {
	id := uuid.New().String()

	create := r.entClient.Client().InsurancePolicy.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetName(name).
		SetCreateTime(time.Now())

	for _, opt := range opts {
		opt(create)
	}

	entity, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, assetV1.ErrorBadRequest("insurance policy '%s' already exists", name)
		}
		r.log.Errorf("create insurance policy failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create insurance policy failed")
	}
	return entity, nil
}

func (r *InsurancePolicyRepo) GetByID(ctx context.Context, id string) (*ent.InsurancePolicy, error) {
	entity, err := r.entClient.Client().InsurancePolicy.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get insurance policy failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get insurance policy failed")
	}
	return entity, nil
}

func (r *InsurancePolicyRepo) List(ctx context.Context, tenantID uint32, page, pageSize int, filters map[string]interface{}) ([]*ent.InsurancePolicy, int, error) {
	query := r.entClient.Client().InsurancePolicy.Query().Where(insurancepolicy.TenantID(tenantID))

	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where(insurancepolicy.Status(status))
	}
	if coverageType, ok := filters["coverage_type"].(string); ok && coverageType != "" {
		query = query.Where(insurancepolicy.CoverageType(coverageType))
	}
	if q, ok := filters["query"].(string); ok && q != "" {
		query = query.Where(
			insurancepolicy.Or(
				insurancepolicy.NameContains(q),
				insurancepolicy.PolicyNumberContains(q),
				insurancepolicy.ProviderContains(q),
			),
		)
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count insurance policies failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list insurance policies failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Asc(insurancepolicy.FieldName)).All(ctx)
	if err != nil {
		r.log.Errorf("list insurance policies failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list insurance policies failed")
	}

	return entities, total, nil
}

func (r *InsurancePolicyRepo) Update(ctx context.Context, id string, updates map[string]interface{}) (*ent.InsurancePolicy, error) {
	update := r.entClient.Client().InsurancePolicy.UpdateOneID(id)

	if name, ok := updates["name"].(string); ok {
		update = update.SetName(name)
	}
	if policyNumber, ok := updates["policy_number"].(string); ok {
		update = update.SetPolicyNumber(policyNumber)
	}
	if provider, ok := updates["provider"].(string); ok {
		update = update.SetProvider(provider)
	}
	if coverageType, ok := updates["coverage_type"].(string); ok {
		update = update.SetCoverageType(coverageType)
	}
	if premiumAmount, ok := updates["premium_amount"].(float64); ok {
		update = update.SetPremiumAmount(premiumAmount)
	}
	if deductible, ok := updates["deductible"].(float64); ok {
		update = update.SetDeductible(deductible)
	}
	if coverageLimit, ok := updates["coverage_limit"].(float64); ok {
		update = update.SetCoverageLimit(coverageLimit)
	}
	if notes, ok := updates["notes"].(string); ok {
		update = update.SetNotes(notes)
	}
	if status, ok := updates["status"].(string); ok {
		update = update.SetStatus(status)
	}
	if validFrom, ok := updates["valid_from"].(time.Time); ok {
		update = update.SetValidFrom(validFrom)
	}
	if validTo, ok := updates["valid_to"].(time.Time); ok {
		update = update.SetValidTo(validTo)
	}

	update = update.SetUpdateTime(time.Now())

	entity, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, assetV1.ErrorInsurancePolicyNotFound("insurance policy not found")
		}
		r.log.Errorf("update insurance policy failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("update insurance policy failed")
	}
	return entity, nil
}

func (r *InsurancePolicyRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().InsurancePolicy.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorInsurancePolicyNotFound("insurance policy not found")
		}
		r.log.Errorf("delete insurance policy failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete insurance policy failed")
	}
	return nil
}

func (r *InsurancePolicyRepo) GetAssetCount(ctx context.Context, policyID string) (int, error) {
	count, err := r.entClient.Client().InsurancePolicyAsset.Query().
		Where(insurancepolicyasset.PolicyID(policyID)).
		Count(ctx)
	if err != nil {
		r.log.Errorf("count policy assets failed: %s", err.Error())
		return 0, assetV1.ErrorInternalServerError("count policy assets failed")
	}
	return count, nil
}

func (r *InsurancePolicyRepo) HasAssets(ctx context.Context, policyID string) (bool, error) {
	count, err := r.GetAssetCount(ctx, policyID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *InsurancePolicyRepo) AddAsset(ctx context.Context, tenantID uint32, policyID, assetID string, coveredValue *float64, notes, assetTag, assetName, modelName string) (*ent.InsurancePolicyAsset, error) {
	id := uuid.New().String()

	create := r.entClient.Client().InsurancePolicyAsset.Create().
		SetID(id).
		SetPolicyID(policyID).
		SetAssetID(assetID).
		SetTenantID(tenantID).
		SetAssetTag(assetTag).
		SetAssetName(assetName).
		SetModelName(modelName).
		SetCreateTime(time.Now())

	if coveredValue != nil {
		create = create.SetCoveredValue(*coveredValue)
	}
	if notes != "" {
		create = create.SetNotes(notes)
	}

	entity, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, assetV1.ErrorAssetAlreadyInPolicy("asset is already in this policy")
		}
		r.log.Errorf("add asset to policy failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("add asset to policy failed")
	}
	return entity, nil
}

func (r *InsurancePolicyRepo) RemoveAsset(ctx context.Context, policyID, assetID string) error {
	_, err := r.entClient.Client().InsurancePolicyAsset.Delete().
		Where(
			insurancepolicyasset.PolicyID(policyID),
			insurancepolicyasset.AssetID(assetID),
		).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("remove asset from policy failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("remove asset from policy failed")
	}
	return nil
}

func (r *InsurancePolicyRepo) ListAssets(ctx context.Context, policyID string) ([]*ent.InsurancePolicyAsset, error) {
	entities, err := r.entClient.Client().InsurancePolicyAsset.Query().
		Where(insurancepolicyasset.PolicyID(policyID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list policy assets failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("list policy assets failed")
	}
	return entities, nil
}
