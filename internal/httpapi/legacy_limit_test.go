package httpapi

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// 032 security review F-1: a legacy cursor/limit request is always bounded
// to 1..listquery.MaxPageSize (default legacyDefaultLimit), and walking the
// legacy cursor still reaches every row.

func TestLegacyLimitClamp(t *testing.T) {
	for v, want := range map[string]int{
		"": legacyDefaultLimit, "x": legacyDefaultLimit, "0": legacyDefaultLimit, "-5": legacyDefaultLimit,
		"7": 7, "200": listquery.MaxPageSize, "500": listquery.MaxPageSize, "100000": listquery.MaxPageSize,
	} {
		q := url.Values{"cursor": {"c"}}
		if v != "" {
			q.Set("limit", v)
		}
		if got := legacyLimit(q); got != want {
			t.Errorf("legacyLimit(%q) = %d, want %d", v, got, want)
		}
	}
}

// walkLegacy follows the legacy id cursor (the last item's id) with the base
// query (may be empty: cursor only), asserting every page holds at most max
// items, and returns the ids seen.
func walkLegacy(t *testing.T, get func(string) map[string]any, base string, max int) []string {
	t.Helper()
	with := func(cursor string) string {
		c := "cursor=" + url.QueryEscape(cursor)
		if base == "" {
			return c
		}
		return base + "&" + c
	}
	var ids []string
	seen := map[string]bool{}
	path := with("")
	for range 100 {
		raw := items(get(path))
		if len(raw) > max {
			t.Fatalf("%s: %d items > %d", path, len(raw), max)
		}
		if len(raw) == 0 {
			return ids
		}
		var last string
		for _, x := range raw {
			last = x.(map[string]any)["id"].(string)
			if seen[last] {
				t.Fatalf("%s: %s twice", path, last)
			}
			seen[last] = true
			ids = append(ids, last)
		}
		path = with(last)
	}
	t.Fatal("legacy walk did not end")
	return nil
}

func TestLegacyListsBounded(t *testing.T) {
	f := newAPI(t, false)
	ctx := context.Background()
	const n = 230
	for i := range n {
		for _, err := range []error{
			f.mem.CreateAsset(ctx, store.Asset{ID: fmt.Sprintf("0190f7c2-0000-7000-8000-%012d", i), TenantID: apiTenant,
				AssetTag: fmt.Sprintf("T-%03d", i), Name: fmt.Sprintf("asset %d", i), Status: store.AssetDeployable}),
			f.mem.CreateSupplier(ctx, store.Supplier{TenantID: apiTenant, Name: fmt.Sprintf("Supplier %d", i)}),
			f.mem.CreateConsumable(ctx, store.Consumable{TenantID: apiTenant, Name: fmt.Sprintf("Toner %d", i)}),
			f.mem.CreateLicense(ctx, store.License{TenantID: apiTenant, Name: fmt.Sprintf("License %d", i)}),
			f.mem.CreateInsurance(ctx, store.InsurancePolicy{ID: fmt.Sprintf("0190f7c2-0001-7000-8000-%012d", i), TenantID: apiTenant,
				Name: fmt.Sprintf("Policy %d", i), PolicyNumber: fmt.Sprintf("P%d", i)}),
		} {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, list := range []string{"/assets", "/suppliers", "/consumables", "/licenses", "/insurance-policies"} {
		get := func(q string) map[string]any { return want(t, f.req(t, "GET", p+list+"?"+q, "admin", ""), 200) }
		// Out-of-range limits are rejected at the edge; the handler clamps
		// whatever it is handed.
		for _, l := range []string{"100000", "0", "-5"} {
			w := f.req(t, "GET", p+list+"?limit="+l, "admin", "")
			if w.Code != 422 && len(items(decodeBody(t, w))) > listquery.MaxPageSize {
				t.Fatalf("%s?limit=%s = %d", list, l, w.Code)
			}
		}
		if m := get("limit=500"); len(items(m)) != listquery.MaxPageSize || m["total"] != float64(n) {
			t.Fatalf("%s?limit=500: %d items, total %v", list, len(items(m)), m["total"])
		}
		if m := get("cursor="); len(items(m)) != legacyDefaultLimit {
			t.Fatalf("%s?cursor=: %d items", list, len(items(m)))
		}
		if ids := walkLegacy(t, get, "limit=500", listquery.MaxPageSize); len(ids) != n {
			t.Fatalf("%s limit=500 walk reached %d", list, len(ids))
		}
		if ids := walkLegacy(t, get, "", legacyDefaultLimit); len(ids) != n {
			t.Fatalf("%s cursor walk reached %d", list, len(ids))
		}
	}
}
