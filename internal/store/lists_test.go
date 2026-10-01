package store

import (
	"testing"

	"github.com/go-tangra/go-tangra/v4/listquery"
)

func TestListSpecs(t *testing.T) {
	for name, s := range map[string]listquery.Spec{"assets": AssetList, "suppliers": SupplierList, "consumables": ConsumableList, "licenses": LicenseList,
		"insurance": InsuranceList, "policy assets": PolicyAssetList, "assignments": AssignmentList, "search": DocumentSearchList} {
		if err := s.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if r := ListRequest(listquery.Request{}, AssetList); r != (listquery.Request{Page: 1, PageSize: 25, Sort: "asset_tag", Order: listquery.Asc}) {
		t.Fatalf("zero request = %+v", r)
	}
	if r := ListRequest(listquery.Request{Page: 2, PageSize: 10, Sort: "assigned_at"}, AssignmentList); r != (listquery.Request{Page: 2, PageSize: 10, Sort: "assigned_at", Order: listquery.Desc}) {
		t.Fatalf("partial request = %+v", r)
	}
	if r := ListRequest(listquery.Request{Page: 3, PageSize: 500, Sort: "nope"}, SupplierList); r != (listquery.Request{Page: 1, PageSize: 25, Sort: "name", Order: listquery.Asc}) {
		t.Fatalf("invalid request = %+v", r)
	}
	if _, err := listquery.New(1, SearchWindow+1, "", "", DocumentSearchList); err == nil {
		t.Fatal("search pages beyond the window")
	}
	for _, c := range []struct {
		r    listquery.Request
		want string
	}{
		{listquery.Request{Sort: "category", Order: listquery.Asc}, "lower(c.name) ASC NULLS LAST, a.id ASC"},
		{listquery.Request{Sort: "location", Order: listquery.Desc}, "lower(COALESCE(NULLIF(l.path, ''), l.name)) DESC NULLS LAST, a.id DESC"},
		{listquery.Request{Sort: "warranty_end", Order: listquery.Asc}, WarrantyEndExpr + " ASC NULLS LAST, a.id ASC"},
	} {
		if got := c.r.OrderBy(AssetList); got != c.want {
			t.Fatalf("%s order = %s", c.r.Sort, got)
		}
	}
}
