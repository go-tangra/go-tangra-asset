package licenses

import (
	"context"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Page is the list-contract page of the tenant's licenses matching f.Query
// (store.LicenseList order).
func (s *Service) Page(ctx context.Context, subj authz.Subjects, f store.ListOpts, req listquery.Request) (listquery.Page[store.License], error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return listquery.Page[store.License]{}, err
	}
	rows, total, applied, err := s.st.PageLicenses(ctx, subj.TenantID, f, req)
	if err != nil {
		return listquery.Page[store.License]{}, err
	}
	return listquery.NewPage(rows, total, applied), nil
}
