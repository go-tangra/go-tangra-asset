package suppliers

import (
	"context"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Page is the list-contract page of the tenant's suppliers matching f.Query
// (store.SupplierList order); contact PII is presented as in List.
func (s *Service) Page(ctx context.Context, subj authz.Subjects, f store.ListOpts, req listquery.Request) (listquery.Page[store.Supplier], error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return listquery.Page[store.Supplier]{}, err
	}
	rows, total, applied, err := s.st.PageSuppliers(ctx, subj.TenantID, f, req)
	if err != nil {
		return listquery.Page[store.Supplier]{}, err
	}
	out := make([]store.Supplier, 0, len(rows))
	for _, sp := range rows {
		out = append(out, s.present(sp, subj))
	}
	return listquery.NewPage(out, total, applied), nil
}
