package insurance

import (
	"context"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Page is the list-contract page of the tenant's policies matching f.Query
// (store.InsuranceList order).
func (s *Service) Page(ctx context.Context, subj authz.Subjects, f store.ListOpts, req listquery.Request) (listquery.Page[store.InsurancePolicy], error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return listquery.Page[store.InsurancePolicy]{}, err
	}
	rows, total, applied, err := s.st.PageInsurance(ctx, subj.TenantID, f, req)
	if err != nil {
		return listquery.Page[store.InsurancePolicy]{}, err
	}
	return listquery.NewPage(rows, total, applied), nil
}

// PolicyAssetsPage is the list-contract page of the assets a policy covers
// (store.PolicyAssetList order).
func (s *Service) PolicyAssetsPage(ctx context.Context, subj authz.Subjects, policyID string, req listquery.Request) (listquery.Page[store.PolicyAsset], error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return listquery.Page[store.PolicyAsset]{}, err
	}
	if _, err := s.st.GetInsurance(ctx, subj.TenantID, policyID); err != nil {
		return listquery.Page[store.PolicyAsset]{}, mapErr(err)
	}
	rows, total, applied, err := s.st.PagePolicyAssets(ctx, subj.TenantID, policyID, req)
	if err != nil {
		return listquery.Page[store.PolicyAsset]{}, err
	}
	return listquery.NewPage(rows, total, applied), nil
}
