package store

import (
	"strings"
	"testing"
	"unicode/utf8"

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
		{listquery.Request{Sort: "purchase_date", Order: listquery.Desc}, "a.purchase_date DESC NULLS LAST, a.id DESC"},
		// NOT NULL columns: no NULLS clause, so (tenant_id, col, id) serves both directions.
		{listquery.Request{Sort: "asset_tag", Order: listquery.Desc}, "lower(a.asset_tag) DESC, a.id DESC"},
		{listquery.Request{Sort: "name", Order: listquery.Asc}, "lower(a.name) ASC, a.id ASC"},
		{listquery.Request{Sort: "status", Order: listquery.Desc}, "a.status DESC, a.id DESC"},
		{listquery.Request{Sort: "created_at", Order: listquery.Desc}, "a.created_at DESC, a.id DESC"},
	} {
		if got := c.r.OrderBy(AssetList); got != c.want {
			t.Fatalf("%s order = %s", c.r.Sort, got)
		}
	}
	for _, c := range []struct {
		s    listquery.Spec
		r    listquery.Request
		want string
	}{
		{AssignmentList, listquery.Request{Sort: "assigned_at", Order: listquery.Desc}, "assigned_at DESC, id DESC"},
		{AssignmentList, listquery.Request{Sort: "returned_at", Order: listquery.Desc}, "returned_at DESC NULLS LAST, id DESC"},
		{LicenseList, listquery.Request{Sort: "valid_to", Order: listquery.Asc}, "valid_to ASC NULLS LAST, id ASC"},
		{ConsumableList, listquery.Request{Sort: "amount", Order: listquery.Desc}, "amount DESC, id DESC"},
		{InsuranceList, listquery.Request{Sort: "created_at", Order: listquery.Desc}, "p.created_at DESC, p.id DESC"},
		{PolicyAssetList, listquery.Request{Sort: "name", Order: listquery.Desc}, "lower(asset_name) DESC, id DESC"},
	} {
		if got := c.r.OrderBy(c.s); got != c.want {
			t.Fatalf("%s order = %s", c.r.Sort, got)
		}
	}
}

func TestTrimQuery(t *testing.T) {
	if got := TrimQuery("abc"); got != "abc" {
		t.Fatalf("short query = %q", got)
	}
	long := strings.Repeat("ж", MaxQueryLen+5)
	if got := TrimQuery(long); utf8.RuneCountInString(got) != MaxQueryLen || !utf8.ValidString(got) {
		t.Fatalf("long query trimmed to %d runes", utf8.RuneCountInString(got))
	}
}
