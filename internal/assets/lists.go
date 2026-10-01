package assets

import (
	"context"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Page is the list-contract page of the tenant's assets matching f
// (store.AssetList order; f.CursorID / f.Limit are ignored).
func (s *Service) Page(ctx context.Context, subj authz.Subjects, f store.AssetFilter, req listquery.Request) (listquery.Page[View], error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return listquery.Page[View]{}, err
	}
	rows, total, applied, err := s.st.PageAssets(ctx, subj.TenantID, f, req)
	if err != nil {
		return listquery.Page[View]{}, err
	}
	out := make([]View, 0, len(rows))
	for _, a := range rows {
		out = append(out, s.view(ctx, a))
	}
	return listquery.NewPage(out, total, applied), nil
}

// AssignmentsPage is the list-contract page of an asset's assignment history
// (store.AssignmentList order: newest first by default).
func (s *Service) AssignmentsPage(ctx context.Context, subj authz.Subjects, id string, req listquery.Request) (listquery.Page[store.Assignment], error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return listquery.Page[store.Assignment]{}, err
	}
	if _, err := s.st.GetAsset(ctx, subj.TenantID, id); err != nil {
		return listquery.Page[store.Assignment]{}, mapErr(err)
	}
	rows, total, applied, err := s.st.PageAssignments(ctx, subj.TenantID, id, req)
	if err != nil {
		return listquery.Page[store.Assignment]{}, err
	}
	return listquery.NewPage(rows, total, applied), nil
}
