package backup

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/memstore"
)

// FuzzImportParser: arbitrary JSON documents never panic the validator or the
// importer; a document that is not the expected shape is refused before any
// write, and a syntactically valid but semantically bad document is refused or
// imported without touching another tenant.
func FuzzImportParser(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schema_version":1}`))
	f.Add([]byte(`{"schema_version":1,"assets":[{"id":"x","asset_tag":"T","name":"n"}]}`))
	f.Add([]byte(`{"schema_version":1,"categories":[{"id":"a","name":"A","parent_id":"a"}]}`))
	f.Add([]byte(`{"schema_version":1,"policy_assets":[{"id":"p","policy_id":"x","asset_id":"y"}]}`))
	f.Add([]byte(`[1,2,3]`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		var b Backup
		if err := json.Unmarshal(raw, &b); err != nil {
			return
		}
		_ = Validate(b)
		mem := memstore.New()
		s := New(mem, nil)
		subj := authz.Subjects{TenantID: "11111111-1111-7111-8111-111111111111", UserID: "u", Roles: []string{"platform-admin"}, ActorKind: authz.ActorUser}
		res, err := s.Import(context.Background(), subj, b, Options{})
		if err != nil {
			return
		}
		if all, _ := mem.AllAssets(context.Background(), "22222222-2222-7222-8222-222222222222"); len(all) != 0 {
			t.Fatal("import wrote into another tenant")
		}
		for _, n := range res.Imported {
			if n < 0 {
				t.Fatal("negative count")
			}
		}
	})
}
