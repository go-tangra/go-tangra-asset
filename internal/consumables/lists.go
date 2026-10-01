package consumables

import (
	"context"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Page is the list-contract page of the tenant's consumables matching f.Query
// (store.ConsumableList order).
func (s *Service) Page(ctx context.Context, subj authz.Subjects, f store.ListOpts, req listquery.Request) (listquery.Page[View], error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return listquery.Page[View]{}, err
	}
	rows, total, applied, err := s.st.PageConsumables(ctx, subj.TenantID, f, req)
	if err != nil {
		return listquery.Page[View]{}, err
	}
	out := make([]View, 0, len(rows))
	for _, c := range rows {
		out = append(out, view(c))
	}
	return listquery.NewPage(out, total, applied), nil
}
