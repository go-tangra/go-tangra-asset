package httpapi

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-tangra/go-tangra-asset/v4/internal/assets"
	"github.com/go-tangra/go-tangra-asset/v4/internal/backup"
	"github.com/go-tangra/go-tangra-asset/v4/internal/categories"
	"github.com/go-tangra/go-tangra-asset/v4/internal/consumables"
	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
	"github.com/go-tangra/go-tangra-asset/v4/internal/insurance"
	"github.com/go-tangra/go-tangra-asset/v4/internal/licenses"
	"github.com/go-tangra/go-tangra-asset/v4/internal/locations"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
	"github.com/go-tangra/go-tangra-asset/v4/internal/suppliers"
	"github.com/go-tangra/go-tangra-asset/v4/internal/userdir"
)

const prefix = "/api/asset/v1"

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func listOpts(r *http.Request) store.ListOpts {
	q := r.URL.Query()
	return store.ListOpts{Query: q.Get("query"), Limit: atoiDefault(q.Get("limit"), 0), CursorID: q.Get("cursor")}
}

// handle wraps a handler that needs the caller's subject.
func (s *Server) handle(method, path string, fn func(w http.ResponseWriter, r *http.Request, subj subjectsT)) {
	s.MustHandle(method, path, func(w http.ResponseWriter, r *http.Request) {
		subj, err := subjects(r)
		if err != nil {
			failSvc(w, err)
			return
		}
		fn(w, r, subj)
	})
}

// Register mounts the asset HTTP routes declared in the OpenAPI document.
// Every route resolves the caller from the gateway-forwarded platform token,
// derives a tenant-scoped subject, and delegates to a domain service. No
// response carries object-store credentials or sealed contact PII.
func (s *Server) Register(d Deps) {
	p := prefix
	s.registerAssets(d)
	s.registerOrg(d)
	s.registerInventories(d)
	s.registerDocuments(d)

	// ---- Users (assignee directory)
	s.handle("GET", p+"/users", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if d.Users == nil {
			WriteJSON(w, http.StatusOK, map[string]any{"items": []userdir.User{}})
			return
		}
		items, err := d.Users.ListUsers(r.Context(), subj.TenantID)
		if err != nil {
			WriteError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
			return
		}
		if q := r.URL.Query().Get("query"); q != "" {
			items = filterUsers(items, q)
		}
		if items == nil {
			items = []userdir.User{}
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	// ---- System
	s.MustHandle("GET", p+"/health", func(w http.ResponseWriter, r *http.Request) {
		out := map[string]any{"status": "ok"}
		if d.Health != nil {
			comps := d.Health()
			out["components"] = comps
			for _, v := range comps {
				if v != "ok" {
					out["status"] = "degraded"
				}
			}
		}
		WriteJSON(w, http.StatusOK, out)
	})
	s.handle("GET", p+"/stats", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		v, err := d.Stats.Get(r.Context(), subj)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})

	// ---- Backup
	s.handle("POST", p+"/backup/export", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in struct{}
		if r.ContentLength > 0 {
			if err := DecodeJSON(r, &in, 0); err != nil {
				Fail(w, r, nil, err)
				return
			}
		}
		b, err := d.Backup.Export(r.Context(), subj)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, b)
	})
	s.handle("POST", p+"/backup/import", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in struct {
			Mode     string        `json:"mode"`
			TenantID string        `json:"tenant_id"`
			Full     bool          `json:"full"`
			Backup   backup.Backup `json:"backup"`
		}
		if err := DecodeJSON(r, &in, MaxImportBytes); err != nil {
			Fail(w, r, nil, err)
			return
		}
		res, err := d.Backup.Import(r.Context(), subj, in.Backup, backup.Options{Mode: in.Mode, TenantID: in.TenantID, Full: in.Full})
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, res)
	})

	// ---- Realtime SSE stream (optional: only when a hub is wired).
	if d.Hub != nil {
		s.RegisterStream(d.Hub)
	}
}

// MaxImportBytes bounds a backup import body (matches the route's declared
// x-freya-max-body-bytes).
const MaxImportBytes = 32 << 20

func filterUsers(items []userdir.User, q string) []userdir.User {
	out := []userdir.User{}
	for _, u := range items {
		if containsFold(u.DisplayName, q) || containsFold(u.ID, q) {
			out = append(out, u)
		}
	}
	return out
}

func (s *Server) registerAssets(d Deps) {
	p := prefix
	s.handle("GET", p+"/assets", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		q := r.URL.Query()
		items, err := d.Assets.List(r.Context(), subj, store.AssetFilter{
			Status: q.Get("status"), CategoryID: q.Get("category_id"), SupplierID: q.Get("supplier_id"), LocationID: q.Get("location_id"),
			UserID: q.Get("user_id"), Query: q.Get("query"), Limit: atoiDefault(q.Get("limit"), 0), CursorID: q.Get("cursor"),
		})
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("POST", p+"/assets", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in assets.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Assets.Create(r.Context(), subj, in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, v)
	})
	s.handle("GET", p+"/assets/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		v, err := d.Assets.Get(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("PUT", p+"/assets/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in assets.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Assets.Update(r.Context(), subj, r.PathValue("id"), in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("DELETE", p+"/assets/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if err := d.Assets.Delete(r.Context(), subj, r.PathValue("id")); err != nil {
			failSvc(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	s.handle("POST", p+"/assets/{id}/assign", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in struct {
			UserID string `json:"user_id"`
			Notes  string `json:"notes"`
		}
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Assets.Assign(r.Context(), subj, r.PathValue("id"), in.UserID, in.Notes)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("POST", p+"/assets/{id}/unassign", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in struct {
			LocationID string `json:"location_id"`
			Notes      string `json:"notes"`
		}
		if r.ContentLength > 0 {
			if err := DecodeJSON(r, &in, 0); err != nil {
				Fail(w, r, nil, err)
				return
			}
		}
		v, err := d.Assets.Unassign(r.Context(), subj, r.PathValue("id"), in.LocationID, in.Notes)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("GET", p+"/assets/{id}/assignments", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Assets.Assignments(r.Context(), subj, r.PathValue("id"), atoiDefault(r.URL.Query().Get("limit"), 0))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	// ---- Inventory sync
	s.handle("POST", p+"/assets/inventory-sync/preview", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if d.Sync == nil {
			WriteError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
			return
		}
		v, err := d.Sync.Preview(r.Context(), subj)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("POST", p+"/assets/inventory-sync/execute", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if d.Sync == nil {
			WriteError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
			return
		}
		var in struct {
			Hostnames []string `json:"hostnames"`
		}
		if r.ContentLength > 0 {
			if err := DecodeJSON(r, &in, 1<<20); err != nil {
				Fail(w, r, nil, err)
				return
			}
		}
		v, err := d.Sync.Execute(r.Context(), subj, in.Hostnames)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
}

func (s *Server) registerOrg(d Deps) {
	p := prefix
	// ---- Categories
	s.handle("GET", p+"/categories", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Categories.List(r.Context(), subj)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("GET", p+"/categories/tree", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Categories.Tree(r.Context(), subj)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("POST", p+"/categories", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in categories.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Categories.Create(r.Context(), subj, in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, v)
	})
	s.handle("GET", p+"/categories/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		v, err := d.Categories.Get(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("PUT", p+"/categories/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in categories.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Categories.Update(r.Context(), subj, r.PathValue("id"), in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("DELETE", p+"/categories/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if err := d.Categories.Delete(r.Context(), subj, r.PathValue("id")); err != nil {
			failSvc(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// ---- Suppliers
	s.handle("GET", p+"/suppliers", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Suppliers.List(r.Context(), subj, listOpts(r))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("POST", p+"/suppliers", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in suppliers.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Suppliers.Create(r.Context(), subj, in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, v)
	})
	s.handle("GET", p+"/suppliers/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		v, err := d.Suppliers.Get(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("PUT", p+"/suppliers/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in suppliers.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Suppliers.Update(r.Context(), subj, r.PathValue("id"), in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("DELETE", p+"/suppliers/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if err := d.Suppliers.Delete(r.Context(), subj, r.PathValue("id")); err != nil {
			failSvc(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// ---- Locations
	s.handle("GET", p+"/locations", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Locations.List(r.Context(), subj)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("GET", p+"/locations/tree", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Locations.Tree(r.Context(), subj)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("POST", p+"/locations", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in locations.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Locations.Create(r.Context(), subj, in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, v)
	})
	s.handle("GET", p+"/locations/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		v, err := d.Locations.Get(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("PUT", p+"/locations/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in locations.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Locations.Update(r.Context(), subj, r.PathValue("id"), in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("DELETE", p+"/locations/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if err := d.Locations.Delete(r.Context(), subj, r.PathValue("id")); err != nil {
			failSvc(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (s *Server) registerInventories(d Deps) {
	p := prefix
	// ---- Consumables
	s.handle("GET", p+"/consumables", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Consumables.List(r.Context(), subj, listOpts(r))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("POST", p+"/consumables", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in consumables.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Consumables.Create(r.Context(), subj, in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, v)
	})
	s.handle("GET", p+"/consumables/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		v, err := d.Consumables.Get(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("PUT", p+"/consumables/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in consumables.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Consumables.Update(r.Context(), subj, r.PathValue("id"), in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("DELETE", p+"/consumables/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		id := r.PathValue("id")
		var docs []store.Document
		if d.Documents != nil {
			docs, _ = d.Documents.List(r.Context(), subj, store.EntityConsumable, id)
		}
		if err := d.Consumables.Delete(r.Context(), subj, id); err != nil {
			failSvc(w, err)
			return
		}
		if d.Documents != nil {
			d.Documents.PurgeRows(r.Context(), subj.TenantID, docs)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// ---- Licenses
	s.handle("GET", p+"/licenses", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Licenses.List(r.Context(), subj, listOpts(r))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("POST", p+"/licenses", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in licenses.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Licenses.Create(r.Context(), subj, in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, v)
	})
	s.handle("GET", p+"/licenses/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		v, err := d.Licenses.Get(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("PUT", p+"/licenses/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in licenses.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Licenses.Update(r.Context(), subj, r.PathValue("id"), in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("DELETE", p+"/licenses/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		id := r.PathValue("id")
		var docs []store.Document
		if d.Documents != nil {
			docs, _ = d.Documents.List(r.Context(), subj, store.EntityLicense, id)
		}
		if err := d.Licenses.Delete(r.Context(), subj, id); err != nil {
			failSvc(w, err)
			return
		}
		if d.Documents != nil {
			d.Documents.PurgeRows(r.Context(), subj.TenantID, docs)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// ---- Insurance
	s.handle("GET", p+"/insurance-policies", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Insurance.List(r.Context(), subj, listOpts(r))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("POST", p+"/insurance-policies", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in insurance.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Insurance.Create(r.Context(), subj, in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, v)
	})
	s.handle("GET", p+"/insurance-policies/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		v, err := d.Insurance.Get(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("PUT", p+"/insurance-policies/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in insurance.Input
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Insurance.Update(r.Context(), subj, r.PathValue("id"), in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("DELETE", p+"/insurance-policies/{id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if err := d.Insurance.Delete(r.Context(), subj, r.PathValue("id")); err != nil {
			failSvc(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	s.handle("GET", p+"/insurance-policies/{id}/assets", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		items, err := d.Insurance.ListPolicyAssets(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	s.handle("POST", p+"/insurance-policies/{id}/assets", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		var in struct {
			AssetID      string  `json:"asset_id"`
			CoveredValue float64 `json:"covered_value"`
			Notes        string  `json:"notes"`
		}
		if err := DecodeJSON(r, &in, 0); err != nil {
			Fail(w, r, nil, err)
			return
		}
		v, err := d.Insurance.AddAssetToPolicy(r.Context(), subj, r.PathValue("id"), in.AssetID, in.CoveredValue, in.Notes)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, v)
	})
	s.handle("DELETE", p+"/insurance-policies/{id}/assets/{asset_id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if err := d.Insurance.RemoveAssetFromPolicy(r.Context(), subj, r.PathValue("id"), r.PathValue("asset_id")); err != nil {
			failSvc(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// registerDocuments mounts the photo route and the per-entity document routes.
func (s *Server) registerDocuments(d Deps) {
	p := prefix
	// ---- Photo
	s.handle("GET", p+"/assets/{id}/photo", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		rc, mime, err := d.Documents.GetPhoto(r.Context(), subj, r.PathValue("id"))
		if err != nil {
			failSvc(w, err)
			return
		}
		defer func() { _ = rc.Close() }()
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Disposition", "inline")
		_, _ = io.Copy(w, rc)
	})
	s.handle("POST", p+"/assets/{id}/photo", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		in, closer, ok := multipartFile(w, r, d.Documents.MaxSize())
		if !ok {
			return
		}
		defer closer()
		v, err := d.Documents.UploadPhoto(r.Context(), subj, r.PathValue("id"), in)
		if err != nil {
			failSvc(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, v)
	})
	s.handle("DELETE", p+"/assets/{id}/photo", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
		if err := d.Documents.DeletePhoto(r.Context(), subj, r.PathValue("id")); err != nil {
			failSvc(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// ---- Documents per entity
	for _, e := range []struct{ typ, route string }{
		{store.EntityAsset, "/assets"}, {store.EntityConsumable, "/consumables"}, {store.EntityLicense, "/licenses"},
	} {
		typ := e.typ
		base := p + e.route + "/{id}/documents"
		s.handle("GET", base, func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
			items, err := d.Documents.List(r.Context(), subj, typ, r.PathValue("id"))
			if err != nil {
				failSvc(w, err)
				return
			}
			WriteJSON(w, http.StatusOK, map[string]any{"items": items})
		})
		s.handle("POST", base, func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
			in, closer, ok := multipartFile(w, r, d.Documents.MaxSize())
			if !ok {
				return
			}
			defer closer()
			in.Description = r.FormValue("description")
			v, err := d.Documents.Upload(r.Context(), subj, typ, r.PathValue("id"), in)
			if err != nil {
				failSvc(w, err)
				return
			}
			WriteJSON(w, http.StatusCreated, v)
		})
		s.handle("DELETE", base+"/{doc_id}", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
			if err := d.Documents.Delete(r.Context(), subj, typ, r.PathValue("id"), r.PathValue("doc_id")); err != nil {
				failSvc(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		s.handle("GET", base+"/{doc_id}/download", func(w http.ResponseWriter, r *http.Request, subj subjectsT) {
			rc, doc, err := d.Documents.Download(r.Context(), subj, typ, r.PathValue("id"), r.PathValue("doc_id"))
			if err != nil {
				failSvc(w, err)
				return
			}
			defer func() { _ = rc.Close() }()
			mime := doc.MimeType
			if mime == "" {
				mime = "application/octet-stream"
			}
			w.Header().Set("Content-Type", mime)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(doc.FileName)+"\"")
			_, _ = io.Copy(w, rc)
		})
	}
}

// multipartFile parses the "file" part of a multipart upload. On refusal it
// writes the response and returns ok=false.
func multipartFile(w http.ResponseWriter, r *http.Request, maxUpload int64) (documents.UploadInput, func(), bool) {
	// The body is already bounded by the route's x-freya-max-body-bytes
	// (http.MaxBytesReader in validate); the in-memory part cap is small so a
	// large upload streams through a temp file rather than RAM.
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil { // #nosec G120 -- body bounded above
		WriteError(w, http.StatusBadRequest, "malformed_body")
		return documents.UploadInput{}, nil, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "file_required")
		return documents.UploadInput{}, nil, false
	}
	if header.Size > maxUpload {
		_ = file.Close()
		WriteError(w, http.StatusRequestEntityTooLarge, "body_too_large")
		return documents.UploadInput{}, nil, false
	}
	return documents.UploadInput{FileName: header.Filename, MimeType: header.Header.Get("Content-Type"), Size: header.Size, Reader: io.LimitReader(file, maxUpload)},
		func() { _ = file.Close() }, true
}
