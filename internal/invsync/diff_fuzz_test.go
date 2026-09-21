package invsync

import (
	"testing"

	"github.com/go-freya/freya/services/asset/internal/invclient"
	"github.com/go-freya/freya/services/asset/internal/store"
)

// FuzzDiff asserts the host↔asset diff never panics and yields exactly one
// change per host with a valid action, for arbitrary host/asset field values.
func FuzzDiff(f *testing.F) {
	f.Add("pc-1", "SN1", "Dell", "h1", "pc-1", "sn1", "h1", "Latitude")
	f.Add("", "", "", "", "", "", "", "")
	f.Add("a", "S", "m", "id", "A", "s", "id", "")
	f.Fuzz(func(t *testing.T, hn, hs, hm, hid, an, as, ahid, am string) {
		hosts := []invclient.Host{{ID: hid, Hostname: hn, SystemSerial: hs, Model: hm}, {ID: hid + "x", Hostname: hn, SystemSerial: hs}}
		assets := []store.Asset{
			{ID: "1", Name: an, Serial: as, ModelName: am, Tags: map[string]string{TagInventoryHostID: ahid}},
			{ID: "2", Name: an, Serial: as, Tags: nil},
			{ID: "3"},
		}
		ch := Diff(hosts, assets)
		if len(ch) != len(hosts) {
			t.Fatalf("%d changes for %d hosts", len(ch), len(hosts))
		}
		seen := map[string]bool{}
		for _, c := range ch {
			switch c.Action {
			case ActionCreate, ActionUpdate, ActionUnchanged:
			default:
				t.Fatalf("bad action %q", c.Action)
			}
			if c.Action != ActionCreate {
				if c.AssetID == "" {
					t.Fatal("matched change without asset id")
				}
				if seen[c.AssetID] {
					t.Fatal("one asset claimed by two hosts")
				}
				seen[c.AssetID] = true
			}
		}
	})
}
